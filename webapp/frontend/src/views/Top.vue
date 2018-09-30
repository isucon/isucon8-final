<template>
  <div class='content'>
    <div class="chart">
      <Price />
    </div>
    <div class="sidebar"></div>
  </div>
</template>

<script lang="ts">
import Vue from 'vue'
import Price from '@/components/Price.vue'

export default Vue.extend({
  name: 'home',
  components: {
    Price,
  },
})
</script>

<style lang="sass" scoped>
.content
  display: flex
  justify-content: center
  align-items: center
  padding-top: 24px

.chart
  margin-right: 24px

.sidebar
  width: 200px
</style>
