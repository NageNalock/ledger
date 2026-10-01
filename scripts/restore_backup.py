#!/usr/bin/env python3
"""Validate and restore a Ledger backup into a NEW data directory."""
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import sqlite3
import stat
import tempfile
import zipfile


def restore(archive, destination):
    destination = Path(destination).absolute()
    if destination.exists():
        raise ValueError('目标目录必须不存在，禁止覆盖现有数据')
    destination.parent.mkdir(parents=True, exist_ok=True)
    stage = Path(tempfile.mkdtemp(prefix='.ledger-restore-', dir=destination.parent))
    try:
        with zipfile.ZipFile(archive) as z:
            names = z.namelist()
            if len(names) != len(set(names)):
                raise ValueError('备份含重复文件')
            manifest_info = z.getinfo('manifest.json')
            if manifest_info.file_size > 16 * 1024 * 1024:
                raise ValueError('备份清单过大')
            manifest = json.loads(z.read('manifest.json'))
            if not isinstance(manifest, dict):
                raise ValueError('备份清单格式错误')
            if manifest.get('format') != 'ledger-backup' or manifest.get('version') != 1:
                raise ValueError('不支持的备份格式')
            hashes = manifest.get('sha256', {})
            if not isinstance(hashes, dict) or not all(isinstance(k, str) and isinstance(v, str) for k, v in hashes.items()):
                raise ValueError('备份校验和格式错误')
            if 'ledger.db' not in hashes or set(names) != set(hashes) | {'manifest.json'}:
                raise ValueError('备份文件与清单不一致')
            total = 0
            for name, expected in hashes.items():
                path = PurePosixPath(name)
                if (path.is_absolute() or '..' in path.parts or '\\' in name
                        or str(path) != name
                        or not (name == 'ledger.db' or name.startswith('uploads/'))):
                    raise ValueError('备份含非法路径')
                entry = z.getinfo(name)
                mode = entry.external_attr >> 16
                if stat.S_ISLNK(mode) or entry.is_dir():
                    raise ValueError('备份含非法文件类型')
                total += entry.file_size
                if total > 20 * 1024**3:
                    raise ValueError('备份超过 20 GiB 解压上限')
                target = stage / name
                target.parent.mkdir(parents=True, exist_ok=True)
                digest = hashlib.sha256()
                with z.open(entry) as src, target.open('xb') as dst:
                    os.chmod(target, 0o600)
                    for block in iter(lambda: src.read(1024 * 1024), b''):
                        digest.update(block)
                        dst.write(block)
                if digest.hexdigest() != expected:
                    raise ValueError(f'文件校验失败：{name}')
        conn = sqlite3.connect((stage / 'ledger.db').as_uri() + '?mode=ro', uri=True)
        try:
            if conn.execute('PRAGMA quick_check').fetchall() != [('ok',)]:
                raise ValueError('数据库完整性检查失败')
            tables = {row[0] for row in conn.execute("SELECT name FROM sqlite_master WHERE type='table'")}
            if not {'users', 'transactions'}.issubset(tables):
                raise ValueError('不是 Ledger 数据库')
        finally:
            conn.close()
        (stage / 'uploads').mkdir(exist_ok=True)
        # mkdir atomically refuses an existing target; never replace active data.
        destination.mkdir(mode=0o700)
        try:
            for item in stage.iterdir():
                shutil.move(str(item), destination / item.name)
        except Exception:
            shutil.rmtree(destination)
            raise
    finally:
        shutil.rmtree(stage)
    return destination


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('archive', type=Path)
    parser.add_argument('destination', type=Path, help='New, non-existing data directory')
    args = parser.parse_args()
    try:
        result = restore(args.archive, args.destination)
    except (ValueError, OSError, KeyError, sqlite3.Error, zipfile.BadZipFile) as exc:
        parser.exit(1, f'恢复失败：{exc}\n')
    print(f'已恢复到 {result}。启动服务前配置 DB_PATH，并将 uploads 放到服务工作目录。')
