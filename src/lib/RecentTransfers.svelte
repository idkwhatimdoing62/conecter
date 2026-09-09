<script>
  export let state = 'loading';
  export let transfers = [];
  export let onRetry = () => {};
  const formatSize = (bytes) => bytes < 1024 ? `${bytes} B` : bytes < 1048576 ? `${(bytes / 1024).toFixed(1)} KB` : bytes < 1073741824 ? `${(bytes / 1048576).toFixed(1)} MB` : `${(bytes / 1073741824).toFixed(2)} GB`;
</script>

<section class="recent"><p>最近传输</p>{#if state === 'loading'}<div class="item"><span class="state-icon loading-dot"><i class="ri-loader-4-line"></i></span><b>正在读取记录</b></div>{:else if state === 'error'}<div class="item recent-error"><span class="state-icon"><i class="ri-error-warning-line"></i></span><b>记录暂时不可用</b><button type="button" onclick={onRetry}>重试</button></div>{:else if !transfers.length}<div class="item"><span class="state-icon"><i class="ri-inbox-line"></i></span><b>还没有传输记录</b><small>上传文件后会显示在这里</small></div>{:else}{#each transfers.slice(0, 5) as share (share.id)}{#each share.files as file (file.name)}<div class="item"><span class="state-icon"><i class="ri-file-3-line"></i></span><b>{file.name}</b><small>{formatSize(file.size)} · 交换码 {share.id}</small></div>{/each}{/each}{/if}</section>
