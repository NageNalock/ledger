const baseUrl = process.env.API_BASE || 'http://localhost:8080/api'
const username = String(process.env.API_USER || '').trim()
const password = String(process.env.API_PASS || '').trim()
const count = Number(process.env.SEED_COUNT || process.argv[2] || 50)
const days = Number(process.env.SEED_DAYS || process.argv[3] || 90)
const projectIdArg = process.env.PROJECT_ID || process.argv[4] || ''
const seedTag = process.env.SEED_TAG || 'seed:test'

const seedMonth = String(process.env.SEED_MONTH || '').trim()
const seedStart = String(process.env.SEED_START || '').trim()
const seedEnd = String(process.env.SEED_END || '').trim()
const seedUsers = String(process.env.SEED_USERS || '').trim()
const perUserCount = Number(process.env.SEED_PER_USER || '') || count
const userPassword = String(process.env.SEED_USER_PASS || '').trim()

const merchants = ['星小喵咖啡', '暖暖食堂', '风铃便利店', '云朵超市', '小橘外卖', '青柠餐厅', '猫爪面包房', '薄荷书店', '云朵花店', '星河影院']
const sources = ['wechat', 'alipay', 'cash', 'bank']
const remarks = ['', '朋友聚餐', '日常补给', '交通出行', '项目采购', '临时支出', '礼物']

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

const randomItem = (arr) => arr[Math.floor(Math.random() * arr.length)]

const endOfMonth = (d) => {
  const x = new Date(d)
  x.setMonth(x.getMonth() + 1)
  x.setDate(0)
  x.setHours(23, 59, 59, 999)
  return x
}

const randomDateBetween = (start, end) => {
  const startMs = start.getTime()
  const endMs = end.getTime()
  const t = startMs + Math.random() * Math.max(1, endMs - startMs)
  const d = new Date(t)
  return d.toISOString()
}

const buildRandomDate = () => {
  if (seedMonth) {
    const start = new Date(`${seedMonth}-01T00:00:00`)
    const end = endOfMonth(start)
    return randomDateBetween(start, end)
  }
  if (seedStart && seedEnd) {
    return randomDateBetween(new Date(`${seedStart}T00:00:00`), new Date(`${seedEnd}T23:59:59`))
  }
  const now = new Date()
  const offset = Math.floor(Math.random() * days)
  const date = new Date(now.getTime() - offset * 24 * 60 * 60 * 1000)
  date.setHours(Math.floor(Math.random() * 23), Math.floor(Math.random() * 59), Math.floor(Math.random() * 59))
  return date.toISOString()
}

const ensureUserExists = async (adminHeaders, uname) => {
  if (!userPassword) {
    throw new Error('Missing SEED_USER_PASS')
  }
  const all = await request('/users', { headers: adminHeaders })
  const existing = Array.isArray(all) ? all.find((u) => String(u?.username || '') === uname) : null
  if (existing) return existing
  const created = await request('/users', {
    method: 'POST',
    headers: adminHeaders,
    body: JSON.stringify({ username: uname, password: userPassword })
  })
  return created
}

const getTokenForUser = async (uname, pass) => {
  const data = await request('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username: uname, password: pass })
  })
  return data.token
}

const main = async () => {
  const adminToken = await getToken()
  const adminHeaders = { Authorization: `Bearer ${adminToken}` }

  const categories = await request('/categories', { headers: adminHeaders })
  const projects = await request('/projects', { headers: adminHeaders })

  if (!Array.isArray(categories) || categories.length === 0) {
    throw new Error('分类数据为空')
  }

  const pairs = []
  for (const l1 of categories) {
    const list = Array.isArray(l1?.children) ? l1.children : []
    for (const l2 of list) {
      pairs.push({ l1, l2 })
    }
  }
  if (!pairs.length) {
    throw new Error('分类二级数据为空（需要至少一个二级分类）')
  }

  const selectedUsers = []
  if (seedUsers) {
    for (const uname of seedUsers.split(',').map((s) => s.trim()).filter(Boolean)) {
      const u = await ensureUserExists(adminHeaders, uname)
      selectedUsers.push({ username: u.username, password: userPassword })
    }
  } else {
    selectedUsers.push({ username, password })
  }

  let totalWritten = 0
  for (const u of selectedUsers) {
    const token = await getTokenForUser(u.username, u.password)
    const headers = { Authorization: `Bearer ${token}` }
    const transactions = []
    for (let i = 0; i < perUserCount; i += 1) {
      const { l1, l2 } = randomItem(pairs)
      const amount = Number((Math.random() * 380 + 20).toFixed(2))
      const projectId = projectIdArg || (projects.length ? randomItem(projects).id : undefined)

      const note = randomItem(remarks)
      const payload = {
        transaction_time: buildRandomDate(),
        merchant: randomItem(merchants),
        amount,
        category_l1_id: l1.id,
        category_l2_id: l2.id,
        source: randomItem(sources),
        remark: note ? `${seedTag} [${u.username}] - ${note}` : `${seedTag} [${u.username}]`
      }

      if (projectId) {
        payload.project_id = projectId
      }

      transactions.push(payload)
    }

    for (const payload of transactions) {
      await request('/transactions', {
        method: 'POST',
        headers,
        body: JSON.stringify(payload)
      })
    }
    totalWritten += transactions.length
  }

  const monthHint = seedMonth ? `（月份=${seedMonth}）` : seedStart && seedEnd ? `（范围=${seedStart}~${seedEnd}）` : `（最近${days}天内随机）`
  const userHint = seedUsers ? `用户=${seedUsers} 每人=${perUserCount}` : `用户=${username} 数量=${perUserCount}`
  console.log(`已写入 ${totalWritten} 条测试流水 ${monthHint} ${userHint}`)
}

main().catch((err) => {
  console.error(err.message || err)
  process.exit(1)
})
