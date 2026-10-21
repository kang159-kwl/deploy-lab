import './style.css'

const app = document.querySelector('#app')

async function loadHealth() {
  const res = await fetch('/api/health')
  const body = await res.json()
  return { ok: res.ok, body }
}
// 测试测试测试
app.innerHTML = `
  <main>
    <h1>deploy-lab</h1>
    <p class="hint">一个用来练部署的最小全栈项目。</p>
    <button id="check">检查后端健康</button>
    <pre id="out">点击按钮请求 /api/health</pre>
  </main>
`

document.querySelector('#check').addEventListener('click', async () => {
  const out = document.querySelector('#out')
  out.textContent = '请求中…'
  try {
    const { ok, body } = await loadHealth()
    out.textContent = (ok ? '健康\n' : '异常\n') + JSON.stringify(body, null, 2)
  } catch (err) {
    out.textContent = '请求失败：' + err.message
  }
})
