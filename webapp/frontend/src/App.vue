<template>
  <div id="app">
    <Header />
    <div id="content">
      <router-view/>
    </div>
    <Modal v-if="isModalOpen"/>
  </div>
</template>

<script lang="ts">
import Vue from 'vue'
import { mapState } from 'vuex'
import Header from '@/components/Header.vue'
import Modal from '@/components/Modal.vue'

export default Vue.extend({
  name: 'App',

  components: {
    Header,
    Modal,
  },

  computed: {
    ...mapState(['isModalOpen']),
  },
})
</script>

<style lang="sass">
#app
  font-family: 'Avenir', Helvetica, Arial, sans-serif
  -webkit-font-smoothing: antialiased
  -moz-osx-font-smoothing: grayscale
  text-align: center
  color: #2c3e50

#content
  padding-top: 64px
</style>
