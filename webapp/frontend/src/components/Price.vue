<template>
  <div class="list">
    <div class="item">
      最安売値
      <span class="price">$300</span>
    </div>
    <div class="item">
      最高買値
      <span class="price">$300</span>
    </div>
  </div>
</template>

<script lang="ts">
import Vue from 'vue'

export default Vue.extend({
  name: 'Price',
})
</script>

<style lang="sass" scoped>
.list
  display: flex
  justify-content: center
  align-items: center
  margin-bottom: 24px

.item
  margin: 0 12px
  font-size: 16px
  font-weight: 500

.price
  text-decoration: underline  

</style>
