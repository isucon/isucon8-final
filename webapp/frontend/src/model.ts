export type ModalType = 'signup' | 'signin'

export type ChartType = 'hour' | 'min' | 'sec'

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
