import './style.css'

const app = document.querySelector('#app')

app.innerHTML = `
  <main>
    <h1>deploy-lab</h1>
    <p class="hint">一个用来练部署的最小全栈项目。</p>

    <section>
      <button id="check">检查后端健康</button>
      <pre id="health-out">点击按钮请求 /api/health</pre>
    </section>

    <section>
      <h2>Notes</h2>
      <form id="note-form">
        <input id="note-title" type="text" placeholder="标题" required />
        <textarea id="note-content" placeholder="内容"></textarea>
        <button type="submit">新建</button>
      </form>
      <pre id="note-error" class="error" hidden></pre>
      <ul id="note-list"></ul>
    </section>
  </main>
`

// ---- 健康检查 ----

document.querySelector('#check').addEventListener('click', async () => {
  const out = document.querySelector('#health-out')
  out.textContent = '请求中…'
  try {
    const res = await fetch('/api/health')
    const body = await res.json()
    out.textContent = (res.ok ? '健康\n' : '异常\n') + JSON.stringify(body, null, 2)
  } catch (err) {
    out.textContent = '请求失败：' + err.message
  }
})

// ---- Notes CRUD ----

const noteList = document.querySelector('#note-list')
const noteForm = document.querySelector('#note-form')
const noteError = document.querySelector('#note-error')
let editingId = null

async function request(path, options) {
  const res = await fetch(path, options)
  let body = null
  try {
    body = await res.json()
  } catch {
    // 204 No Content 没有 body
  }
  if (!res.ok) {
    throw new Error((body && body.message) || `请求失败（${res.status}）`)
  }
  return body
}

function showError(err) {
  noteError.hidden = false
  noteError.textContent = err.message
}

function clearError() {
  noteError.hidden = true
  noteError.textContent = ''
}

function renderNotes(notes) {
  noteList.innerHTML = notes.map((note) => {
    if (note.id === editingId) {
      return `
        <li data-id="${note.id}">
          <input class="edit-title" type="text" value="${escapeHtml(note.title)}" />
          <textarea class="edit-content">${escapeHtml(note.content)}</textarea>
          <div class="actions">
            <button data-action="save">保存</button>
            <button data-action="cancel">取消</button>
          </div>
        </li>
      `
    }
    return `
      <li data-id="${note.id}">
        <div class="note-body">
          <strong>${escapeHtml(note.title)}</strong>
          <p>${escapeHtml(note.content)}</p>
        </div>
        <div class="actions">
          <button data-action="edit">编辑</button>
          <button data-action="delete">删除</button>
        </div>
      </li>
    `
  }).join('') || '<li class="empty">还没有笔记</li>'
}

function escapeHtml(str) {
  return String(str ?? '').replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]))
}

async function loadNotes() {
  try {
    const notes = await request('/api/notes')
    clearError()
    renderNotes(notes)
  } catch (err) {
    showError(err)
  }
}

noteForm.addEventListener('submit', async (e) => {
  e.preventDefault()
  const title = document.querySelector('#note-title')
  const content = document.querySelector('#note-content')
  try {
    await request('/api/notes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: title.value, content: content.value }),
    })
    title.value = ''
    content.value = ''
    clearError()
    await loadNotes()
  } catch (err) {
    showError(err)
  }
})

noteList.addEventListener('click', async (e) => {
  const action = e.target.dataset.action
  if (!action) return
  const li = e.target.closest('li')
  const id = li.dataset.id

  try {
    if (action === 'edit') {
      editingId = Number(id)
      await loadNotes()
    } else if (action === 'cancel') {
      editingId = null
      await loadNotes()
    } else if (action === 'delete') {
      await request(`/api/notes/${id}`, { method: 'DELETE' })
      clearError()
      await loadNotes()
    } else if (action === 'save') {
      const title = li.querySelector('.edit-title').value
      const content = li.querySelector('.edit-content').value
      await request(`/api/notes/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ title, content }),
      })
      editingId = null
      clearError()
      await loadNotes()
    }
  } catch (err) {
    showError(err)
  }
})

loadNotes()
