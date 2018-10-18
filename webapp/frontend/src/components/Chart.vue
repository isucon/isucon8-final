<template>
  <div class="container">
    <div class="buttons">
      <button class="button" :data-selected='chartType === "hour"' @click="setChartType('hour')">Hour</button>
      <button class="button" :data-selected='chartType === "min"' @click="setChartType('min')">Minute</button>
      <button class="button" :data-selected='chartType === "sec"' @click="setChartType('sec')">Second</button>
    </div>
    <canvas id="chart" ref='canvas'/>
  </div>
</template>

<script lang="ts">
import Vue from 'vue'
import { mapState, mapMutations } from 'vuex'
import { ChartData } from '../model'

declare const moment: any
declare const Chart: any

interface ConvertedData {
  c: number
  h: number
  l: number
  o: number
  t: number
}

interface Data {
  ctx: CanvasRenderingContext2D | null
}

const convertDataStructure = (data: ChartData[]): ConvertedData[] => {
  return data.map((d) => {
    return {
      c: d.close,
      h: d.high,
      l: d.low,
      o: d.open,
      t: moment(d.time).valueOf() as number,
    }
  })
}

export default Vue.extend({
  name: 'Chart',

  data(): Data {
    return {
      ctx: null,
    }
  },

  computed: {
    ...mapState(['chartType', 'info']),
  },

  methods: {
    ...mapMutations(['setChartType']),
    getChartData() {
      if (!this.info) { return }
      return this.chartType === 'hour' ? this.info.chart_by_hour
        : this.chartType === 'min' ? this.info.chart_by_min
        : this.chartType === 'sec' ? this.info.chart_by_sec
        : null
    },
    setupContext2d() {
      const canvas = this.$refs.canvas as HTMLCanvasElement
      this.ctx = canvas.getContext('2d')
      if (!this.ctx) { return }

      this.ctx.canvas.width = 900
      this.ctx.canvas.height = 400
    },
    showChart() {
      if (!this.info) { return }

      const candlestickChart = new Chart(
        this.ctx,
        {
          type: 'candlestick',
          data: {
            datasets: [{
              label: 'ISUCOIN Chart',
              data: convertDataStructure(this.getChartData()),
            }],
          },
        },
      )
    },
  },

  mounted() {
    this.setupContext2d()
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

  &[data-selected='true']
    background-color: rgba(0,0,0,0.24)
    pointer-events: none

  &:hover
    background-color: rgba(0,0,0,0.12)

  &:active
    background-color: rgba(0,0,0,0.24)

#chart
  width: 100%
  height: 400px
  pointer-events: none

</style>
