<template>
  <canvas id="chart" ref='canvas'/>
</template>

<script lang="ts">
import Vue from 'vue'

declare const Chart: any

export default Vue.extend({
  name: 'Chart',
  mounted() {
    const canvas = this.$refs.canvas as HTMLCanvasElement
    const ctx = canvas.getContext('2d')
    if (!ctx) {
      return
    }

    ctx.canvas.width = 500
    ctx.canvas.height = 200

    const candlestickChart = new Chart(
      ctx,
      {
        type: 'candlestick',
        data: {
          datasets: [{
            label: 'Candlestick Chart',
            data: [
              {
                c: 30.100933174779865,
                h: 32.022643756279706,
                l: 26.531181315088716,
                o: 29.27681922687249,
                t: 1490972400000,
              },
              {
                c: 29.247213124979055,
                h: 32.61784236874654,
                l: 28.560871909708275,
                o: 30.342204678707052,
                t: 1493305200000,
              },
              {
                c: 30.23371122198542,
                h: 32.60326452930426,
                l: 28.176437196518908,
                o: 30.594227465677033,
                t: 1494514800000,
              },
              {
                c: 32.17631969339388,
                h: 32.55490771874663,
                l: 31.71272224600758,
                o: 32.33802548156477,
                t: 1496934000000,
              },
            ],
          }],
        },
      },
    )
  },
})
</script>

<style lang="sass" scoped>
#chart
  width: 500px
  height: 200px

</style>
