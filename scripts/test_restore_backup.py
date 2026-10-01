import hashlib
import json
from pathlib import Path
import sqlite3
import tempfile
import unittest
import zipfile
from restore_backup import restore


class RestoreTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        db = self.root / 'source.db'
        with sqlite3.connect(db) as conn:
            conn.executescript("CREATE TABLE users(id INTEGER, password_hash TEXT); INSERT INTO users VALUES(1, 'hash'); CREATE TABLE transactions(id INTEGER, amount REAL); INSERT INTO transactions VALUES(1, 25.5);")
        self.files = {'ledger.db': db.read_bytes(), 'uploads/a.png': b'image'}

    def archive(self, files=None, corrupt=False):
        files = files or self.files
        manifest = {'format': 'ledger-backup', 'version': 1, 'sha256': {k: hashlib.sha256(v).hexdigest() for k, v in files.items()}}
        archive = self.root / 'backup.zip'
        with zipfile.ZipFile(archive, 'w') as z:
            for k, v in files.items():
                z.writestr(k, v + b'corrupt' if corrupt else v)
            z.writestr('manifest.json', json.dumps(manifest))
        return archive

    def test_roundtrip_and_no_overwrite(self):
        target = self.root / 'restored'
        restore(self.archive(), target)
        self.assertEqual((target / 'uploads/a.png').read_bytes(), b'image')
        with sqlite3.connect(target / 'ledger.db') as conn:
            self.assertEqual(conn.execute('SELECT amount FROM transactions').fetchone()[0], 25.5)
            self.assertEqual(conn.execute('SELECT password_hash FROM users').fetchone()[0], 'hash')
        with self.assertRaises(ValueError):
            restore(self.archive(), target)

    def test_corruption_leaves_no_target(self):
        target = self.root / 'restored'
        with self.assertRaises(ValueError):
            restore(self.archive(corrupt=True), target)
        self.assertFalse(target.exists())

    def test_path_traversal_rejected(self):
        for name in ['uploads/../../escape', '/escape', 'uploads/../escape', 'uploads\\escape']:
            with self.subTest(name=name), self.assertRaises(ValueError):
                restore(self.archive({**self.files, name: b'bad'}), self.root / 'restored')
        self.assertFalse((self.root / 'escape').exists())

    def test_invalid_database_rejected(self):
        with self.assertRaises(sqlite3.DatabaseError):
            restore(self.archive({**self.files, 'ledger.db': b'invalid'}), self.root / 'restored')


if __name__ == '__main__':
    unittest.main()
