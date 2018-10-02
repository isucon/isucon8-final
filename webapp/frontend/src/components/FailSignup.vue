<template>
  <div class="error-text">登録に失敗しました</div>
</template>

<script lang="ts">
import Vue from 'vue'

export default Vue.extend({
  name: 'FailSignup',
})
</script>

<style lang="sass" scoped>
.error-text
  font-size: 16px
  color: rgb(255,0,0)
  text-align: center
</style>
