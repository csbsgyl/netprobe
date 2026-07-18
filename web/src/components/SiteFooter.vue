<template>
  <footer class="site-footer">
    <span class="footer-brand">NetProbe</span>
    <span>服务地址取决于当前访问域名；检测结果只代表本次设备、网络和测试时间。</span>
  </footer>
</template>

<style scoped>
.site-footer {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 1120px;
  margin: 0 auto;
  padding: 22px 24px;
  border-top: 1px solid var(--line);
  display: flex;
  justify-content: space-between;
  gap: 12px;
  color: var(--muted);
  font-size: 12.5px;
}

.footer-brand {
  color: var(--ink);
  font-weight: 700;
}

@media (max-width: 560px) {
  .site-footer {
    flex-direction: column;
    gap: 6px;
    padding: 18px 16px;
  }
}
</style>
