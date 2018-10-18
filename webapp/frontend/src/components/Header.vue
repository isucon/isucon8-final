<template>
  <header class='header'>
    <h1 class='title'><img src="/img/isucoin_logo.png" alt="ISUCOIN"></h1>
    <ul class="list" v-if='user'>
      <li class="item" >{{ user.name }}</li>
    </ul>
    <ul class="list" v-else>
      <li class="item" @click="openSignupModal()">Sign up</li>
      <li class="item" @click="openSigninModal()">Sign in</li>
    </ul>
  </header>
</template>

<script lang="ts">
import Vue from 'vue'
import { mapActions, mapMutations, mapState } from 'vuex'

export default Vue.extend({
  name: 'Header',

  computed: {
    ...mapState(['user']),
  },

  methods: {
    ...mapActions(['openSignupModal', 'openSigninModal']),
  },
})
</script>

<style scoped lang="sass">
.header
  display: flex
  justify-content: space-between
  align-items: center
  position: fixed
  top: 0
  left: 0
  width: 100%
  height: 64px
  box-sizing: border-box
  padding: 0 24px
  background-color: #fff
  box-shadow: 0px 2px 4px -1px rgba(0,0,0,0.2), 0px 4px 5px 0px rgba(0,0,0,0.14), 0px 1px 10px 0px rgba(0,0,0,0.12)

  & img
    height: 40px

.title
  margin: 0
  font-size: 20px
  height: 40px 

.list
  display: flex
  align-items: center
  height: 100%
  margin: 0
  padding: 0
  list-style: none

.item
  display: flex
  align-items: center
  height: 100%
  padding: 0 16px
  cursor: pointer
  transition: 0.3s cubic-bezier(0.25, 0.8, 0.5, 1)

  &:hover
    background-color: rgba(0,0,0,0.12)

  &:active
    background-color: rgba(0,0,0,0.24)

</style>
