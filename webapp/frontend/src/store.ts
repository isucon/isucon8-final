import Vue from 'vue'
import Vuex from 'vuex'
import axios from 'axios'

Vue.use(Vuex)

export default new Vuex.Store({
  state: {
    isModalOpen: false,
    modalType: 'signup',
    info: null,
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
    async getInfo({ commit }) {
      try {
        const response = await axios.get('/info')
        commit('setInfo', response.data)
      } catch (error) {
        // tslint:disable
        console.error('failed to fetch /info')
        throw error
      }
    },
  },
})
