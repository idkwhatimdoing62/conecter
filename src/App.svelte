<script>
  import { onMount, tick } from 'svelte';
  import { createApi } from './lib/api.js';
  import RecentTransfers from './lib/RecentTransfers.svelte';

  const API = location.port === '5174' ? `http://${location.hostname}:5173` : '';
  const { request } = createApi();
  let tab = location.pathname.startsWith('/s/') ? 'receive' : 'send';
  let lanIP = location.hostname;
  let files = [];
  let activeShare = null;
  let uploadState = 'idle';
  let uploadProgress = 0;
  let uploadMessage = '分享链接将在 24 小时后失效';
  let isDragging = false;
  let receiveCode = location.pathname.match(/^\/s\/([A-Za-z0-9]+)/)?.[1]?.toUpperCase() || '';
  let receiveState = 'idle';
  let receiveMessage = '';
  let received = null;
  let recentState = 'loading';
  let recent = [];
  let copied = false;
  let toast = '';
  let codeInput;
  let xhr;
  let isRevoking = false;
  let toastTimer;
  const MAX_UPLOAD_BYTES = 5 * 1024 * 1024 * 1024;
  const MAX_UPLOAD_FILES = 50;

  const formatSize = (bytes) => bytes < 1024 ? `${bytes} B` : bytes < 1048576 ? `${(bytes / 1024).toFixed(1)} KB` : bytes < 1073741824 ? `${(bytes / 1048576).toFixed(1)} MB` : `${(bytes / 1073741824).toFixed(2)} GB`;
  const totalSize = () => files.reduce((total, file) => total + file.size, 0);
  const shareUrl = (id) => `${location.protocol}//${lanIP}:${location.port || '5173'}/s/${id}`;
  const showToast = (message) => { toast = message; clearTimeout(toastTimer); toastTimer = setTimeout(() => toast = '', 2200); };

  function setFiles(list) {
    const selected = [...list];
    activeShare = null; uploadProgress = 0;
    if (selected.length > MAX_UPLOAD_FILES) { files = []; uploadState = 'error'; uploadMessage = `一次最多选择 ${MAX_UPLOAD_FILES} 个文件`; return; }
    const size = selected.reduce((total, file) => total + file.size, 0);
    if (size > MAX_UPLOAD_BYTES) { files = []; uploadState = 'error'; uploadMessage = '所选文件总大小超过 5 GB'; return; }
    files = selected; uploadState = files.length ? 'ready' : 'idle'; uploadMessage = files.length ? '确认文件后开始上传' : '分享链接将在 24 小时后失效';
  }
  function choose(event) { setFiles(event.target.files); }
  function drop(event) { event.preventDefault(); isDragging = false; setFiles(event.dataTransfer.files); }
  function dragOver(event) { event.preventDefault(); isDragging = true; }
  function dragLeave(event) { if (event.currentTarget === event.target) isDragging = false; }
  function resetUpload() { xhr?.abort(); files = []; activeShare = null; uploadState = 'idle'; uploadProgress = 0; uploadMessage = '分享链接将在 24 小时后失效'; }

  function startUpload() {
    if (!files.length || uploadState === 'uploading') return;
    const body = new FormData(); files.forEach((file) => body.append('files', file));
    uploadState = 'uploading'; uploadProgress = 0; uploadMessage = '正在准备上传，请保持页面打开';
    xhr = new XMLHttpRequest(); xhr.open('POST', `${API}/api/shares`);
    xhr.upload.onprogress = (event) => { if (event.lengthComputable) { uploadProgress = Math.round(event.loaded / event.total * 100); uploadMessage = `正在上传 ${uploadProgress}% · ${(event.loaded / 1048576).toFixed(1)} / ${(event.total / 1048576).toFixed(1)} MB`; } };
    xhr.onload = () => {
      if (xhr.status < 200 || xhr.status >= 300) { uploadState = 'error'; uploadMessage = xhr.status === 429 ? '请求过于频繁，请稍后再试。' : '上传失败，请重试。'; return; }
      try { activeShare = JSON.parse(xhr.responseText); } catch { uploadState = 'error'; uploadMessage = '服务返回了无效响应，请重试。'; return; }
      if (!activeShare?.id) { uploadState = 'error'; uploadMessage = '服务返回了无效分享信息，请重试。'; return; }
      uploadState = 'done'; uploadProgress = 100; uploadMessage = '上传完成，链接将在 24 小时后失效'; saveRecent({ id: activeShare.id, files: files.map((file) => ({ name: file.name, size: file.size })) }); showToast('分享已生成');
    };
    xhr.onerror = () => { uploadState = 'error'; uploadMessage = '上传失败，请确认 Go 服务正在运行。'; };
    xhr.send(body);
  }

  async function copyText(value) {
    if (navigator.clipboard?.writeText) { try { await navigator.clipboard.writeText(value); return true; } catch {} }
    const helper = document.createElement('textarea'); helper.value = value; helper.style.position = 'fixed'; helper.style.opacity = '0'; document.body.appendChild(helper); helper.select(); const copiedText = document.execCommand('copy'); helper.remove(); return copiedText;
  }
  async function copyCode() { copied = await copyText(activeShare.id); if (copied) showToast('交换码已复制'); setTimeout(() => copied = false, 1600); }
  async function copyLink() { const copiedLink = await copyText(shareUrl(activeShare.id)); if (copiedLink) showToast('分享链接已复制'); }
  async function revoke() {
    if (!activeShare || isRevoking) return; isRevoking = true;
    try { const response = await request(`${API}/api/shares/${activeShare.id}`, { method: 'DELETE', headers: { 'X-Revoke-Token': activeShare.revokeToken || '' } });
      if (response.ok) { const revokedId = activeShare.id; activeShare = null; uploadMessage = '文件已从本机移除'; showToast('分享已撤销'); removeRecent(revokedId); }
      else uploadMessage = '撤销失败，请重试。';
    } catch { uploadMessage = '网络连接失败，请重试。'; } finally { isRevoking = false; }
  }
  async function lookup() {
    if (receiveCode.length !== 6) return; receiveState = 'loading'; receiveMessage = ''; received = null;
    try { const response = await request(`${API}/api/shares/${receiveCode}`); if (!response.ok) { receiveState = 'error'; receiveMessage = response.status === 429 ? '请求过于频繁，请稍后再试。' : '分享不存在或已过期。'; return; } const data = await response.json(); if (!Array.isArray(data.files)) throw new Error(); received = data; receiveState = 'done'; showToast('已找到分享文件'); }
    catch { receiveState = 'error'; receiveMessage = '网络连接失败，请检查 Go 服务。'; }
  }
  function normalizeCode() { receiveCode = receiveCode.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 6); receiveMessage = ''; }
  function switchTab(nextTab) { tab = nextTab; if (nextTab === 'receive') tick().then(() => codeInput?.focus()); }
  function saveRecent(share) { const next = [share, ...recent.filter((item) => item.id !== share.id)].slice(0, 5); recent = next; localStorage.setItem('conecter-recent', JSON.stringify(next)); }
  function removeRecent(id) { if (!id) return; recent = recent.filter((item) => item.id !== id); localStorage.setItem('conecter-recent', JSON.stringify(recent)); }
  async function loadRecent() { recentState = 'loading'; try { recent = JSON.parse(localStorage.getItem('conecter-recent') || '[]'); recentState = 'done'; } catch { recent = []; recentState = 'error'; } }
  async function loadNetworkInfo() { try { const response = await request(`${API}/api/info`, {}, 5000); if (response.ok) lanIP = (await response.json()).ip || lanIP; } catch {} }
  onMount(() => { loadRecent(); loadNetworkInfo(); if (tab === 'receive') tick().then(() => codeInput?.focus()); });
</script>

<div class="app"><a class="skip-link" href="#main-content">跳到主要内容</a>
  <header><div class="brand"><span class="logo"><i class="ri-share-forward-2-line"></i></span><b>conecter</b></div><div class="status"><i></i><span class="status-label">局域网在线</span><span class="status-sep">·</span><span class="status-ip">{lanIP}</span></div></header>
  <main id="main-content"><div class="intro"><p>局域网文件传输</p><h1>文件，<em>直接传。</em></h1><span>不用注册，也不用云盘。<br>生成链接，对方打开即可下载。</span></div>
    <section class="card" aria-label="文件传输"><nav aria-label="传输模式"><button class:on={tab === 'send'} type="button" on:click={() => switchTab('send')}>发送文件</button><button class:on={tab === 'receive'} type="button" on:click={() => switchTab('receive')}>输入密钥</button></nav>
      {#if tab === 'send'}
        <label class:chosen={files.length > 0} class:dragging={isDragging} class="drop" on:dragover={dragOver} on:dragleave={dragLeave} on:drop={drop}><input aria-label="选择要发送的文件" type="file" multiple on:change={choose}><span><i class="ri-upload-cloud-2-line"></i></span><strong>{uploadState === 'uploading' ? '正在上传文件' : files.length ? `${files.length} 个文件已选择` : '拖放文件到这里'}</strong><small>{files.length ? `${files.map((file) => file.name).join(', ')} · 共 ${formatSize(totalSize())}` : '点击选择文件，或将文件拖到这里 · 单次最多 5 GB'}</small>{#if files.length && uploadState === 'ready'}<button class="choose" type="button" on:click|stopPropagation={startUpload}>开始上传 <i class="ri-arrow-right-up-line"></i></button>{/if}{#if uploadState === 'uploading'}<div class="progress" aria-label={`上传进度 ${uploadProgress}%`}><span style={`width:${uploadProgress}%`}></span></div><button id="cancelUpload" type="button" on:click|stopPropagation={resetUpload}>取消上传</button>{/if}</label>
        {#if activeShare}<div class="share success-state"><div class="share-heading"><span class="success-icon"><i class="ri-check-line"></i></span><div><strong>分享已生成</strong><small>对方可以使用交换码或链接打开</small></div></div><small>分享链接</small><div class="url">{shareUrl(activeShare.id)}</div><div class="share-preview"><img src={`${API}/api/shares/${activeShare.id}/qr?url=${encodeURIComponent(shareUrl(activeShare.id))}`} alt="分享二维码"><span>扫码打开分享</span></div><div class="share-actions"><button type="button" on:click={copyLink}><i class="ri-links-line"></i> 复制链接</button><a href={shareUrl(activeShare.id)} target="_blank" rel="noreferrer"><i class="ri-external-link-line"></i> 打开链接</a></div><small>交换码</small><div class="share-row"><b>{activeShare.id}</b><button type="button" on:click={copyCode}><i class="ri-file-copy-line"></i> {copied ? '已复制' : '复制交换码'}</button><button id="revoke" type="button" disabled={isRevoking} on:click={revoke}>{isRevoking ? '正在撤销…' : '撤销分享'}</button></div></div>{:else}<div class="share muted"><small>选择文件后生成分享链接和交换码</small></div>{/if}
        <div class:safe-error={uploadState === 'error'} class="safe">{uploadState === 'error' ? '!' : '◈'} {uploadMessage}</div>
      {:else}
        <div class="receive"><span><i class="ri-link"></i></span><h2>打开一份分享</h2><p>输入分享者发来的 6 位交换码</p><input bind:this={codeInput} bind:value={receiveCode} on:input={normalizeCode} maxlength="6" placeholder="输入 6 位交换码" aria-label="分享交换码"><button id="lookup" class="primary" type="button" disabled={receiveCode.length !== 6 || receiveState === 'loading'} on:click={lookup}>{receiveState === 'loading' ? '正在查找…' : '查看文件'} <i class="ri-arrow-right-up-line"></i></button>{#if receiveMessage}<div class="error" role="alert"><i class="ri-error-warning-line"></i>{receiveMessage}</div>{/if}{#if received}<div class="result-heading"><strong>找到 {received.files.length} 个文件</strong><span>分享码 {received.id}</span></div><div class="download-list">{#each received.files as file}<div><span><i class="ri-file-3-line"></i>{file.name}<small>{formatSize(file.size)}</small></span><a href={`${API}${file.download}`} download>下载</a></div>{/each}</div>{#if received.files.length > 1}<div class="download-actions"><a href={`${API}/api/shares/${received.id}/zip`} download>下载全部 ZIP <i class="ri-download-2-line"></i></a></div>{/if}{/if}</div>
      {/if}
    </section>
    <RecentTransfers state={recentState} transfers={recent} onRetry={loadRecent} />
  </main><footer><span>CONECTER</span><span>数据只在你的局域网内流转</span></footer>
  {#if toast}<div class="toast" role="status"><i class="ri-check-line"></i>{toast}</div>{/if}
</div>




