<template>
  <div>
    <p class="error-message" v-if='hasSigninError'>ログインに失敗しました</p>
    <div class="row">
      bank id
      <input type="text" class="input" v-model='bank_id' autofocus='true'>
    </div>
    <div class="row">
      password
      <input type="password" class="input" v-model='password'>
    </div>
    <button class="button" @click.prevent='postSignin()'>ログイン</button>
  </div>
</template>

<script lang="ts">
import Vue from 'vue'
import { mapActions, mapState, mapMutations } from 'vuex'
import axios from 'axios'

export default Vue.extend({
  name: 'SigninForm',

  data() {
    return {
      bank_id: '',
      password: '',
    }
  },

  computed: {
    ...mapState(['hasSigninError']),
  },

  methods: {
    ...mapActions(['signin', 'getOrders']),
    ...mapMutations(['closeModal', 'showSigninError', 'hideSigninError']),
    async postSignin() {
      const data = {
        bank_id: this.bank_id,
        password: this.password,
      }
      await this.signin(data)
      await this.getOrders()
    },
  },

  watch: {
    bank_id() {
      this.hideSigninError()
    },
    password() {
      this.hideSigninError()
    },
  },
})
</script>

<style lang="sass" scoped>
.error-message
  font-size: 16px
  color: rgb(255,0,0)
  text-align: center

.row
  width: 320px
  margin-bottom: 24px
  font-size: 16px
  font-weight: 500
  color: rgba(0,0,0,0.54)
  text-align: left

  &:last-of-type
    margin-bottom: 48px

.input
  display: block
  width: 100%
  appearance: none
  outline: none
  border: none
  border-bottom: 1px solid #a9a9a9
  font-size: 16px
  line-height: 20px
  transition: 0.3s cubic-bezier(0.25, 0.8, 0.5, 1)

  &:hover
    border-bottom-color: #666

  &:focus, &:active
    border-bottom-color: #1867c0

.button
  appearance: none
  display: block
  margin: 0 auto
  border: none
  outline: none
  padding: 8px 32px
  background-color: rgba(245,245,245,1)
  box-shadow: 0px 3px 1px -2px rgba(0,0,0,0.2), 0px 2px 2px 0px rgba(0,0,0,0.14), 0px 1px 5px 0px rgba(0,0,0,0.12)
  font-size: 15px
  font-weight: 500
  transition: 0.3s cubic-bezier(0.25, 0.8, 0.5, 1)
  cursor: pointer

  &:hover
    background-color: rgba(0,0,0,0.12)

  &:active
    background-color: rgba(0,0,0,0.24)
</style>
