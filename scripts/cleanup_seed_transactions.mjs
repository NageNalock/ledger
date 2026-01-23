const baseUrl = process.env.API_BASE || 'http://localhost:8080/api'
const username = String(process.env.API_USER || '').trim()
const password = String(process.env.API_PASS || '').trim()
const seedTag = process.env.SEED_TAG || 'seed:test'
const startDate = process.env.CLEAN_START || process.argv[2] || ''
const endDate = process.env.CLEAN_END || process.argv[3] || ''
const pageSize = Number(process.env.PAGE_SIZE || 50)

const request = async (path, options = {}) => {
  const res = await fetch(`${baseUrl}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers || {})
    }
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const msg = data?.error || res.statusText
    throw new Error(msg)
  }
  return data
}

const getToken = async () => {
  if (process.env.API_TOKEN) return process.env.API_TOKEN
  if (!username || !password) {
    throw new Error('Missing API_TOKEN or API_USER/API_PASS')
  }
  const data = await request('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password })
  })
  return data.token
}

const shouldDelete = (tx) => {
  const remark = tx?.remark || ''
  return typeof remark === 'string' && remark.startsWith(seedTag)
}

const main = async () => {
  const token = await getToken()
  const headers = { Authorization: `Bearer ${token}` }

  let page = 1
  let totalDeleted = 0

  while (true) {
    const params = new URLSearchParams({
      page: String(page),
      page_size: String(pageSize)
    })

    if (startDate) params.set('start_date', startDate)
    if (endDate) params.set('end_date', endDate)

    const res = await request(`/transactions?${params.toString()}`, { headers })
    const data = res?.data || []

    if (!data.length) break

    for (const tx of data) {
      if (shouldDelete(tx)) {
        await request(`/transactions/${tx.id}`, { method: 'DELETE', headers })
        totalDeleted += 1
      }
    }

    if (data.length < pageSize) break
    page += 1
  }

  console.log(`已清理 ${totalDeleted} 条测试流水`) 
}

main().catch((err) => {
  console.error(err.message || err)
  process.exit(1)
})
