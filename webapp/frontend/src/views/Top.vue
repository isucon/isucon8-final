<template>
  <div class='content'>
    <div class="chart">
      <Price />
      <Chart />
    </div>
    <div class="order">
      <Order />
      <Log />
    </div>
  </div>
</template>

<script lang="ts">
import Vue from 'vue'
import { mapActions, mapState } from 'vuex'
import Price from '@/components/Price.vue'
import Chart from '@/components/Chart.vue'
import Order from '@/components/Order.vue'
import Log from '@/components/Log.vue'

export default Vue.extend({
  name: 'home',

  components: {
    Price,
    Chart,
    Order,
    Log,
  },

  mounted() {
    this.updateInfo()
  },

  computed: {
    ...mapState(['info']),
  },

  methods: {
    ...mapActions(['getInfo', 'getOrders']),
    async updateInfo() {
      try {
        await this.getInfo(this.info ? this.info.cursor : null)
        if (this.info && this.info.traded_orders && this.info.traded_orders.length > 0) {
          this.getOrders()
        }

        setTimeout(() => this.updateInfo(), 1000)
      } catch (error) {
        throw error
      }
    },
  },
})
</script>

<style lang="sass" scoped>
.content
  display: flex
  justify-content: center
  padding-top: 24px

.chart
  margin-right: 24px

.order
  width: 260px
</style>
