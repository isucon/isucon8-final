<template>
  <canvas id="chart" ref='canvas'/>
</template>

<script lang="ts">
import Vue from 'vue'
import { mapState } from 'vuex'

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
    ...mapState(['info']),
  },

  methods: {
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
    showChart() {
      if (!this.info) { return }

      const canvas = this.$refs.canvas as HTMLCanvasElement
      const ctx = canvas.getContext('2d')
      if (!ctx) { return }

      ctx.canvas.width = 700
      ctx.canvas.height = 200

      const candlestickChart = new Chart(
        ctx,
        {
          type: 'candlestick',
          data: {
            datasets: [{
              label: 'ISUCOIN Chart',
              data: this.convertDataStructure(this.info.chart_by_min),
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
    info() {
      this.showChart()
    },
  },
})
</script>

<style lang="sass" scoped>
#chart
  width: 700px
  height: 200px

</style>
