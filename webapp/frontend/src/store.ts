import Vue from 'vue'
import Vuex from 'vuex'
import axios from 'axios'

Vue.use(Vuex)

export default new Vuex.Store({
  state: {
    chartType: 'min',
    hasSigninError: false,
    hasSignupError: false,
    info: null,
    isModalOpen: false,
    modalType: 'signup',
  },
  mutations: {
    openModal(state) {
      state.isModalOpen = true
    },
    closeModal(state) {
      state.isModalOpen = false
    },
    setModalType(state, type) {
      state.modalType = type
    },
    setInfo(state, info) {
      state.info = info
    },
    setChartType(state, type) {
      state.chartType = type
    },
    showSigninError(state) {
       state.hasSigninError = true
    },
    hideSigninError(state) {
      state.hasSigninError = false
    },
    showSignupError(state) {
      state.hasSignupError = true
    },
    hideSignupError(state) {
      state.hasSignupError = false
    },
  },
  actions: {
    openSignupModal({ commit }) {
      commit('setModalType', 'signup')
      commit('openModal')
    },
    openSigninModal({ commit }) {
      commit('setModalType', 'signin')
      commit('openModal')
    },
    async getInfo({ commit }, cursor?) {
      const config = cursor ? { params: { cursor } } : undefined

      try {
        const response = await axios.get('/info', config)
        commit('setInfo', response.data)
      } catch (error) {
        // tslint:disable
        console.error('failed to fetch /info')
        throw error
      }
    },
  },
})
