import './styles.css';

const formatSize = (bytes) => bytes < 1024 ? `${bytes} B` : bytes < 1048576 ? `${(bytes / 1024).toFixed(1)} KB` : bytes < 1073741824 ? `${(bytes / 1048576).toFixed(1)} MB` : `${(bytes / 1073741824).toFixed(2)} GB`;
const escapeHtml = (value) => String(value).replace(/[&<>"']/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char]);
const API = location.port === '5174' ? `http://${location.hostname}:5173` : '';
let xhr;
let lanIP = location.hostname;
let activeShare = null;

document.querySelector('#app').innerHTML = `<div class="app"><header><div class="brand"><span class="logo"><i class="ri-share-forward-2-line"></i></span><b>conecter</b></div><div class="status" id="status"><i></i><span class="status-label">局域网在线</span><span class="status-sep">·</span><span class="status-ip">正在识别…</span></div></header><main><div class="intro"><p>文件传输工具</p><h1>文件，<em>直接传。</em></h1><span>无需注册，无需云盘。<br>生成链接和交换码，对方打开后即可下载。</span></div><section class="card"><nav><button class="on" id="sendTab" type="button">发送文件</button><button id="getTab" type="button">输入密钥</button></nav><div id="content"></div></section><section class="recent" id="recent"><p>最近传输</p><div class="item"><span>…</span><b>正在读取记录</b><small></small></div></section></main><footer><span>CONECTER</span><span>数据只在你的局域网内流转</span></footer></div>`;
const content = document.querySelector('#content');

function shareUrl(id) {
  return `${location.protocol}//${lanIP}:${location.port || '5173'}/s/${id}`;
}

function send() {
  activeShare = null;
  content.innerHTML = `<label class="drop"><input id="picker" type="file" multiple><span><i class="ri-upload-cloud-2-line"></i></span><strong>拖放文件到这里</strong><small>或点击选择 · 最大 5 GB</small></label><div class="share muted"><small>选择文件后生成分享链接和交换码</small></div><div class="safe">◈ 分享链接将在 24 小时后失效</div>`;
  const picker = document.querySelector('#picker');
  picker.addEventListener('change', (event) => startUpload(event.target.files));
  const drop = document.querySelector('.drop');
  ['dragover', 'dragenter'].forEach((eventName) => drop.addEventListener(eventName, (event) => {
    event.preventDefault();
    drop.classList.add('dragging');
  }));
  drop.addEventListener('dragleave', () => drop.classList.remove('dragging'));
  drop.addEventListener('drop', (event) => {
    event.preventDefault();
    drop.classList.remove('dragging');
    startUpload(event.dataTransfer.files);
  });
}

function startUpload(list) {
  if (!list.length) return;
  const body = new FormData();
  [...list].forEach((file) => body.append('files', file));
  const drop = document.querySelector('.drop');
  const safe = document.querySelector('.safe');
  drop.innerHTML = '<strong>正在准备上传…</strong><small>请保持页面打开</small><button id="cancelUpload" type="button">取消上传</button>';
  xhr = new XMLHttpRequest();
  document.querySelector('#cancelUpload').onclick = () => {
    xhr.abort();
    safe.textContent = '◈ 上传已取消';
    send();
  };
  xhr.open('POST', `${API}/api/shares`);
  xhr.upload.onprogress = (event) => {
    if (event.lengthComputable) safe.textContent = `正在上传 ${Math.round(event.loaded / event.total * 100)}% · ${(event.loaded / 1048576).toFixed(1)} / ${(event.total / 1048576).toFixed(1)}`;
  };
  xhr.onload = () => {
    if (xhr.status < 200 || xhr.status >= 300) {
      safe.textContent = xhr.status === 429 ? '请求过于频繁，请稍后再试。' : '上传失败，请重试。';
      return;
    }
    let share;
    try {
      share = JSON.parse(xhr.responseText);
    } catch {
      safe.textContent = '服务返回了无效响应，请重试。';
      return;
    }
    if (!share.id) {
      safe.textContent = '服务返回了无效分享信息，请重试。';
      return;
    }
    const url = shareUrl(share.id);
    activeShare = share;
    document.querySelector('.share').outerHTML = `<div class="share"><small>分享链接</small><div class="url">${escapeHtml(url)}</div><div class="share-preview"><img src="${API}/api/shares/${share.id}/qr?url=${encodeURIComponent(url)}" alt="分享二维码"><span>扫码打开分享</span></div><small>交换码</small><div class="share-row"><b>${share.id}</b><button id="copyCode" type="button" data-code="${share.id}"><i class="ri-file-copy-line"></i> 复制交换码</button><button id="revoke" type="button">撤销分享</button></div></div>`;
    safe.textContent = '◈ 上传完成 · 链接将在 24 小时后失效';
    loadRecent();
  };
  xhr.onerror = () => { safe.textContent = '上传失败，请确认 Go 服务正在运行。'; };
  xhr.send(body);
}

function receive() {
  content.innerHTML = `<div class="receive"><span><i class="ri-link-m"></i></span><h2>打开一份分享</h2><p>输入分享者发来的 6 位交换码</p><input id="code" maxlength="6" placeholder="000000"><button id="lookup" type="button" disabled>查看文件 <i class="ri-arrow-right-up-line"></i></button><div id="result"></div></div>`;
  const input = document.querySelector('#code');
  const button = document.querySelector('#lookup');
  input.oninput = () => {
    input.value = input.value.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 6);
    button.disabled = input.value.length !== 6;
  };
  button.onclick = async () => {
    const result = document.querySelector('#result');
    button.disabled = true;
    result.textContent = '正在查找…';
    try {
      const response = await fetch(`${API}/api/shares/${input.value}`);
      if (!response.ok) {
        result.textContent = response.status === 429 ? '请求过于频繁，请稍后再试。' : '分享不存在或已过期。';
        return;
      }
      const data = await response.json();
      if (!Array.isArray(data.files)) {
        result.textContent = '分享数据无效。';
        return;
      }
      const items = data.files.map((file) => `<div><span>${escapeHtml(file.name)}<small>${formatSize(file.size)}</small></span><a href="${API}${file.download}" download>下载</a></div>`).join('');
      const all = data.files.length > 1 ? `<div class="download-actions"><a href="${API}/api/shares/${data.id}/zip" download>下载全部 ZIP <i class="ri-download-2-line"></i></a></div>` : '';
      result.innerHTML = `<div class="download-list">${items}</div>${all}`;
    } catch {
      result.textContent = '网络连接失败，请检查 Go 服务。';
    } finally {
      button.disabled = input.value.length !== 6;
    }
  };
}

async function copyText(value) {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(value);
      return;
    } catch {
      // HTTP LAN pages may expose the API but deny permission; use the fallback.
    }
  }
  const helper = document.createElement('textarea');
  helper.value = value;
  helper.style.position = 'fixed';
  helper.style.opacity = '0';
  document.body.appendChild(helper);
  helper.select();
  const copied = document.execCommand('copy');
  helper.remove();
  if (!copied) throw new Error('copy failed');
}

document.addEventListener('click', async (event) => {
  const copyButton = event.target.closest('#copyCode');
  if (copyButton) {
    const original = copyButton.innerHTML;
    try {
      await copyText(copyButton.dataset.code);
      copyButton.textContent = '已复制';
      setTimeout(() => { copyButton.innerHTML = original; }, 1600);
    } catch {
      copyButton.textContent = '复制失败';
      setTimeout(() => { copyButton.innerHTML = original; }, 1600);
    }
    return;
  }
  if (event.target.id !== 'revoke' || !activeShare) return;
  event.target.disabled = true;
  const response = await fetch(`${API}/api/shares/${activeShare.id}`, { method: 'DELETE' });
  if (response.ok) {
    activeShare = null;
    document.querySelector('.share').classList.add('muted');
    document.querySelector('.share').innerHTML = '<small>分享已撤销</small>';
    document.querySelector('.safe').textContent = '◈ 文件已从本机移除';
    loadRecent();
  } else {
    event.target.disabled = false;
    document.querySelector('.safe').textContent = '撤销失败，请重试。';
  }
});

document.querySelector('#sendTab').onclick = () => { sendTab.classList.add('on'); getTab.classList.remove('on'); send(); };
document.querySelector('#getTab').onclick = () => { getTab.classList.add('on'); sendTab.classList.remove('on'); receive(); };

async function loadRecent() {
  const element = document.querySelector('#recent');
  if (!element) return;
  element.innerHTML = '<p>最近传输</p><div class="item"><span>…</span><b>正在读取记录</b><small></small></div>';
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 8000);
  try {
    const response = await fetch(`${API}/api/shares`, { signal: controller.signal });
    if (!response.ok) throw new Error('recent request failed');
    const data = await response.json();
    if (!data.length) {
      element.innerHTML = '<p>最近传输</p><div class="item"><span>—</span><b>还没有传输记录</b><small>上传文件后会显示在这里</small></div>';
      return;
    }
    element.innerHTML = '<p>最近传输</p>' + data.slice(0, 5).flatMap((share) => share.files.map((file) => `<div class="item"><span><i class="ri-file-3-line"></i></span><b>${escapeHtml(file.name)}</b><small>${formatSize(file.size)} · 交换码 ${share.id}</small></div>`)).join('');
  } catch {
    element.innerHTML = '<p>最近传输</p><div class="item recent-error"><span>!</span><b>记录暂时不可用</b><button type="button" id="retryRecent">重试</button></div>';
  } finally {
    clearTimeout(timeout);
  }
}

document.addEventListener('click', (event) => { if (event.target.id === 'retryRecent') loadRecent(); });

async function loadNetworkInfo() {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 5000);
  try {
    const response = await fetch(`${API}/api/info`, { signal: controller.signal });
    if (!response.ok) throw new Error('network info failed');
    const data = await response.json();
    lanIP = data.ip || lanIP;
    const element = document.querySelector('.status-ip');
    if (element) element.textContent = lanIP;
  } catch {
    const element = document.querySelector('.status-ip');
    if (element) element.textContent = lanIP || '本机';
  } finally {
    clearTimeout(timeout);
  }
}

send();
loadRecent();
loadNetworkInfo();
