<template>
  <div class="log">
    <h3 class="title">履歴</h3>
    <ul class="orders">
      <li class="order" v-for='order in orders' :key='order.id' :data-type='order.type' :data-traded='isTradedOrder(order) ? "true" : "false"' :data-closed='order.closed_at ? "true" : "false"'>{{ `${getDate(order.created_at)}\n脚数: ${order.amount}, 単価: ${order.price}` }}<button class="cancel" @click.prevent='deleteOrders(order.id)'>×</button></li>
    </ul>
  </div>  
</template>

<script lang="ts">
import Vue from 'vue'
import { mapActions, mapState } from 'vuex'
import axios from 'axios'
import { Order } from '../model'

declare const moment: any

interface Data {
  tradedOrders: Order[]
}

export default Vue.extend({
  name: 'Log',

  data(): Data {
    return {
      tradedOrders: [],
    }
  },

  computed: {
    ...mapState(['info', 'orders']),
  },

  methods: {
    ...mapActions(['getOrders']),
    async deleteOrders(orderId: number) {
      try {
        await axios.delete(`/order/${orderId}`)
        await this.getOrders()
      } catch (error) {
        throw error
      }
    },
    getDate(datestring: string) {
      return moment(datestring).format('YYYY/MM/DD')
    },
    isTradedOrder(order: Order) {
      return this.tradedOrders.filter((tradedOrder: Order) => tradedOrder.id === order.id).length > 0
    },
  },

  watch: {
    info(info) {
      if (info && info.traded_orders && info.traded_orders.length > 0) {
        this.tradedOrders = info.traded_orders
      }
    },
  },
})
</script>

<style lang="sass" scoped>
.log 
  width: 100%
  box-sizing: border-box
  padding: 24px
  background-color: #fff
  box-shadow: 0px 2px 4px -1px rgba(0,0,0,0.2), 0px 4px 5px 0px rgba(0,0,0,0.14), 0px 1px 10px 0px rgba(0,0,0,0.12)

.title
  margin: 0 0 18px
  font-size: 18px
  font-weight: 500
  text-align: left

.orders
  margin: 0
  padding: 0
  list-style: none

.order
  display: flex
  justify-content: space-between
  align-items: center
  margin-bottom: 12px
  font-size: 12px
  white-space: pre

  &[data-type='sell']:before,
  &[data-type='buy']:before
    padding: 2px 4px
    color: #fff
    font-weight: bold

  &[data-type='sell']:before
    background-color: #0068b7
    content: '売り'

  &[data-type='buy']:before
    background-color: #d70035
    content: '買い'

  &[data-traded='true']
    animation: traded-order 3s linear 0s
  
  &[data-closed='true']
    opacity: 0.4
    font-style: italic

@keyframes traded-order
  0%
    background-color: #edde7b
  100%
    background-color: transparent

.cancel
  appearance: none
  display: block
  width: 20px
  height: 20px
  padding: 0
  border: none
  border-radius: 50%
  outline: none
  font-size: 18px
  line-height: 18px
  text-align: center
  color: #666
  cursor: pointer
  transition: 0.3s cubic-bezier(0.25, 0.8, 0.5, 1)

  &:hover
    background-color: rgba(0,0,0,0.12)

  &:active
    background-color: rgba(0,0,0,0.24)
  
  [data-closed='true'] &
    visibility: hidden

</style>
