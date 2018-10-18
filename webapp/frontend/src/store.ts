import Vue from 'vue'
import Vuex from 'vuex'
import axios from 'axios'
import * as Model from '@/model'

Vue.use(Vuex)

const updateChartData = (targetChart: Model.ChartData[], receivedChart: Model.ChartData[]) => {
  receivedChart.forEach((data: Model.ChartData) => {
    const duplicatedData = targetChart.find((element: Model.ChartData) => element.time === data.time)

    if (duplicatedData) {
      targetChart.map((element: Model.ChartData) => {
        return duplicatedData.time === element.time ? data : element
      })
    } else {
      targetChart.push(data)
      targetChart.shift()
    }
  })
}

const initialState: Model.State = {
  chartType: 'min',
  hasSigninError: false,
  hasSignupError: false,
  info: null,
  isModalOpen: false,
  modalType: 'signup',
  orders: [],
  user: null,
}

export default new Vuex.Store({
  state: initialState,
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
    async getInfo({ commit, state }, cursor?) {
      const config = cursor ? { params: { cursor } } : undefined

      try {
        const response = await axios.get('/info', config)
        const info = response.data

        if (state.info === null) {
          commit('setInfo', {
            ...info,
            chart_by_min: info.chart_by_min.splice(-60),
            chart_by_sec: info.chart_by_sec.splice(-60),
          })
          return
        }

        updateChartData(state.info.chart_by_hour, info.chart_by_hour)
        updateChartData(state.info.chart_by_min, info.chart_by_min)
        updateChartData(state.info.chart_by_sec, info.chart_by_sec)

        commit('setInfo', {
          ...info,
          chart_by_hour: state.info.chart_by_hour,
          chart_by_min: state.info.chart_by_min,
          chart_by_sec: state.info.chart_by_sec,
        })
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
          commit('setOrders', response.data as Model.Order[])
        }
      } catch (error) {
        throw error
      }
    },
  },
})
