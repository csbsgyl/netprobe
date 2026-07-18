<script setup lang="ts">
import ParticleField from "./components/ParticleField.vue";
import SiteHeader from "./components/SiteHeader.vue";
import HeroSection from "./components/HeroSection.vue";
import ResultGrid from "./components/ResultGrid.vue";
import DeploymentInfo from "./components/DeploymentInfo.vue";
import SiteFooter from "./components/SiteFooter.vue";
</script>

<template>
  <ParticleField />
  <SiteHeader />
  <main>
    <HeroSection />
    <ResultGrid />
    <DeploymentInfo />
  </main>
  <SiteFooter />
</template>

<style scoped>
main {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 1120px;
  flex: 1;
  margin: 0 auto;
  padding: 0 24px 72px;
}

@media (max-width: 560px) {
  main {
    padding: 0 16px 56px;
  }
}
</style>
