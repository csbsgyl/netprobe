<template>
  <footer class="site-footer">
    <span class="footer-brand">NetProbe</span>
    <span>检测只代表当前设备、当前网络和本次测试时间。</span>
  </footer>
</template>

<style scoped>
.site-footer {
  width: 100%;
  max-width: 1120px;
  margin: 0 auto;
  padding: 24px;
  border-top: 1px solid var(--line);
  display: flex;
  justify-content: space-between;
  gap: 12px;
  color: var(--faint);
  font-size: 12px;
}

.footer-brand {
  color: var(--muted);
  font-weight: 700;
  font-family: var(--mono);
}

@media (max-width: 560px) {
  .site-footer {
    flex-direction: column;
    gap: 6px;
    padding: 20px 18px;
  }
}
</style>
