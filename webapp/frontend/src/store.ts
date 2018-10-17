import Vue from 'vue'
import Vuex from 'vuex'
import axios from 'axios'

Vue.use(Vuex)

export interface User {
  id: number
  name: string
}

export interface Trade {
  id: number
  amount: number
  price: number
  created_at: ''
}

export interface Order {
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

export interface ChartData {
  close: number
  high: number
  low: number
  open: number
  time: string
}

export interface Info {
  chart_by_hour: ChartData[]
  chart_by_min: ChartData[]
  chart_by_sec: ChartData[]
  cursor: number
  enable_share: boolean
  highest_buy_price: number
  lowest_sell_price: number
  traded_orders: Order[]
}

export type ModalType = 'signup' | 'signin'

export type ChartType = 'hour' | 'min' | 'sec'

export interface State {
  chartType: ChartType
  hasSigninError: boolean
  hasSignupError: boolean
  info: Info | null
  isModalOpen: boolean
  modalType: ModalType
  orders: []
  user: User | null
}


const initialState: State = {
  chartType: 'min',
  hasSigninError: false,
  hasSignupError: false,
  info: null,
  isModalOpen: false,
  modalType: 'signup',
  orders: [],
  user: null,
}

const updateChartData = (targetChart: ChartData[], receivedChart: ChartData[]) => {
  receivedChart.forEach((data: ChartData) => {
    const duplicatedData = targetChart.find((element: ChartData) => element.time === data.time)

    if (duplicatedData) {
      targetChart.map((element: ChartData) => {
        return duplicatedData.time === element.time ? data : element
      })
    } else {
      targetChart.push(data)
      targetChart.shift()
    }
  })
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
      if (state.info === null) {
        state.info = info
        return
      }

      updateChartData(state.info.chart_by_hour, info.chart_by_hour)
      updateChartData(state.info.chart_by_min, info.chart_by_min)
      updateChartData(state.info.chart_by_sec, info.chart_by_sec)

      state.info = {
        ...info,
        chart_by_hour: state.info.chart_by_hour,
        chart_by_min: state.info.chart_by_min,
        chart_by_sec: state.info.chart_by_sec,
      }
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
  },
})
