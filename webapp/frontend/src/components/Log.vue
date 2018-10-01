<template>
  <div class="log">
    <h3 class="title">履歴</h3>
    <ul class="orders">
      <li class="order">
        オーダー
        <button class="cancel">×</button>
      </li>
      <li class="order">
        オーダー
        <button class="cancel">×</button>
      </li>
      <li class="order">
        オーダー
        <button class="cancel">×</button>
      </li>
    </ul>
  </div>  
</template>

<script lang="ts">
import Vue from 'vue'

export default Vue.extend({
  name: 'Log',
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
  font-size: 16px

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

</style>
