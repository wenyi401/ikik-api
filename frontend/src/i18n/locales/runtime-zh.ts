import base from './zh'
import modular from './zh/index'
import additions from './runtime-additions-zh'
import { mergeMissingLocaleMessages } from './merge'

export default mergeMissingLocaleMessages(
  mergeMissingLocaleMessages(base, modular),
  additions,
)
