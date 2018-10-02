<template>
  <button class="share" v-if='info && info.enable_share'>Share on ISUBATA</button>
</template>

<script lang="ts">
import Vue from 'vue'
import { mapState } from 'vuex'

export default Vue.extend({
  name: 'ShareButton',

  computed: {
    ...mapState(['info']),
  },
})
</script>

<style lang="sass" scoped>
.share
  appearance: none
  border: none
  outline: none
  padding: 12px
  border-radius: 4px
  background-color: #24140e
  font-size: 14px
  font-weight: 700
  color: #fff
  cursor: pointer
</style>
