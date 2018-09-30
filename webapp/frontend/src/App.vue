<template>
  <div id="app">
    <Header />
    <div id="content">
      <div id="nav">
        <router-link to="/">Top</router-link> |
        <router-link to="/info">Info</router-link>
      </div>
      <router-view/>
    </div>
  </div>
</template>

<script lang="ts">
import Vue from 'vue'
import Header from '@/components/Header.vue'

export default Vue.extend({
  name: 'App',
  components: {
    Header,
  },
})
</script>

<style lang="sass">
#app
  font-family: 'Avenir', Helvetica, Arial, sans-serif
  -webkit-font-smoothing: antialiased
  -moz-osx-font-smoothing: grayscale
  text-align: center
  color: #2c3e50

#content
  padding-top: 64px
</style>
