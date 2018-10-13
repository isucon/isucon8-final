<template>
  <div class="container">
    <div class="buttons">
      <button class="button" @click="setChartType('hour')">Hour</button>
      <button class="button" @click="setChartType('min')">Minute</button>
      <button class="button" @click="setChartType('sec')">Second</button>
    </div>
    <canvas id="chart" ref='canvas'/>
  </div>
</template>

<script lang="ts">
import Vue from 'vue'
import { mapState, mapMutations } from 'vuex'

declare const moment: any
declare const Chart: any

interface ChartData {
  close: number
  high: number
  low: number
  open: number
  time: string
}

interface ConvertedData {
  c: number
  h: number
  l: number
  o: number
  t: number
}

export default Vue.extend({
  name: 'Chart',

  computed: {
    ...mapState(['chartType', 'info']),
  },

  methods: {
    ...mapMutations(['setChartType']),
    convertDataStructure(data: ChartData[]): ConvertedData[] {
      return data.map((d) => {
        return {
          c: d.close,
          h: d.high,
          l: d.low,
          o: d.open,
          t: moment(d.time).valueOf() as number,
        }
      })
    },
    getChartData() {
      if (!this.info) { return }
      return this.chartType === 'hour' ? this.info.chart_by_hour
        : this.chartType === 'min' ? this.info.chart_by_min
        : this.chartType === 'sec' ? this.reduceChartData(this.info.chart_by_sec)
        : null
    },
    reduceChartData(chart: ChartData[]) {
      return chart.filter((data, index) => index < 60 * 3)
    },
    showChart() {
      if (!this.info) { return }

      const canvas = this.$refs.canvas as HTMLCanvasElement
      const ctx = canvas.getContext('2d')
      if (!ctx) { return }

      ctx.canvas.width = 900
      ctx.canvas.height = 400

      const candlestickChart = new Chart(
        ctx,
        {
          type: 'candlestick',
          data: {
            datasets: [{
              label: 'ISUCOIN Chart',
              data: this.convertDataStructure(this.getChartData()),
            }],
          },
        },
      )
    },
  },

  mounted() {
    this.showChart()
  },

  watch: {
    chartType() {
      this.showChart()
    },
    info() {
      this.showChart()
    },
  },
})
</script>

<style lang="sass" scoped>
.container
  width: 900px

.buttons
  display: flex
  justify-content: center
  align-items: center
  margin-bottom: 12px

.button
  appearance: none
  display: block
  border: none
  outline: none
  margin: 0 12px
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

#chart
  width: 100%
  height: 200px

</style>
