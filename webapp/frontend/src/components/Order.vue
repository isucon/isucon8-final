<template>
  <div class="order">
    <div class="row">
      脚数
      <input type="number" class="input" v-model='amount'>
    </div>
    <div class="row">
      単価
      <input type="number" class="input" v-model='price'>
    </div>
    <div class="buttons">
      <button class="button" @click.prevent='sell()'>売り</button>
      <button class="button" @click.prevent='buy()'>買い</button>
    </div>
    <ShareButton v-if='didOrder' />
  </div>
</template>

<script lang="ts">
import Vue from 'vue'
import { mapActions, mapState } from 'vuex'
import axios from 'axios'
import ShareButton from '@/components/ShareButton.vue'

export default Vue.extend({
  name: 'Order',

  components: {
    ShareButton,
  },

  data() {
    return {
      amount: 0,
      didOrder: false,
      price: 0,
    }
  },

  computed: {
    ...mapState(['orders']),
  },

  methods: {
    ...mapActions(['getOrders']),
    async postOrders(type: string) {
      const params = new URLSearchParams()
      params.append('type', type)
      params.append('amount', String(this.amount))
      params.append('price', String(this.price))

      try {
        const response = await axios.post('/orders', params)
        if (response.status === 200) {
          this.didOrder = true
          await this.getOrders()
        }
      } catch (error) {
        throw error
      }
    },
    buy() {
      this.postOrders('buy')
    },
    sell() {
      this.postOrders('sell')
    },
  },
})
</script>

<style lang="sass" scoped>
.order
  width: 100%
  box-sizing: border-box
  margin-bottom: 24px
  padding: 24px
  background-color: #fff
  box-shadow: 0px 2px 4px -1px rgba(0,0,0,0.2), 0px 4px 5px 0px rgba(0,0,0,0.14), 0px 1px 10px 0px rgba(0,0,0,0.12)

.row
  margin-bottom: 12px
  font-size: 16px
  font-weight: 500
  color: rgba(0,0,0,0.54)
  text-align: left

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

.buttons
  display: flex
  justify-content: space-between
  width: 100%

.button
  appearance: none
  display: block
  border: none
  outline: none
  margin-bottom: 24px
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
