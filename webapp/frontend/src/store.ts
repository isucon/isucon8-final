import Vue from 'vue'
import Vuex from 'vuex'
import axios from 'axios'

Vue.use(Vuex)

interface User {
  id: number
  name: string
}

interface Trade {
  id: number
  amount: number
  price: number
  created_at: ''
}

interface Order {
  id: number
  type: string
  user_id: number
  amount: number
  price: number
  closed_at: string | null
  trade_id: number
  created_at: string
  user: User
  trade: Trade
}

export default new Vuex.Store({
  state: {
    chartType: 'min',
    hasSigninError: false,
    hasSignupError: false,
    info: null,
    isModalOpen: false,
    modalType: 'signup',
    orders: [],
    user: null,
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
    setUser(state, user) {
      state.user = user
    },
    setOrders(state, orders) {
      state.orders = orders
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
    async signin({ commit }, { bank_id, password }) {
      const params = new URLSearchParams()
      params.append('bank_id', bank_id)
      params.append('password', password)

      try {
        const response = await axios.post('/signin', params)
        if (response.status === 200) {
          commit('setUser', response.data)
          commit('closeModal')
        }
      } catch (error) {
        commit('showSigninError')
        throw error
      }
    },
    async getOrders({ commit }) {
      try {
        const response = await axios.get('/orders')
        if (response.status === 200) {
          commit('setOrders', response.data as Order[])
        }
      } catch (error) {
        throw error
      }
    },
    async postOrders({ commit }, { type, amount, price }) {
      const params = {
        type,
        amount,
        price
      }

      try {
        await axios.post('/orders', params)
      } catch (error) {
        throw error
      }
    },
    async deleteOrders({ commit }, orderId) {
      const params = {
        id: orderId 
      }

      try {
        await axios.delete('/orders', { params })
      } catch (error) {
        throw error
      }
    }
  },
})
