<script setup lang="ts">
import SiteHeader from "./components/SiteHeader.vue";
import HeroSection from "./components/HeroSection.vue";
import ResultGrid from "./components/ResultGrid.vue";
import DeepTest from "./components/DeepTest.vue";
import SiteFooter from "./components/SiteFooter.vue";
</script>

<template>
  <SiteHeader />
  <main>
    <HeroSection />
    <ResultGrid />
    <DeepTest />
  </main>
  <SiteFooter />
</template>

<style scoped>
main {
  width: 100%;
  max-width: 1120px;
  flex: 1;
  margin: 0 auto;
  padding: 0 24px 88px;
}

@media (max-width: 560px) {
  main {
    padding: 0 18px 64px;
  }
}
</style>
