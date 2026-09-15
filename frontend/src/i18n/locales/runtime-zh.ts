import base from './zh'
import modular from './zh/index'
import additions from './runtime-additions-zh'
import mergeAdditions from './merge-additions-zh'
import { mergeMissingLocaleMessages } from './merge'

export default mergeMissingLocaleMessages(
  mergeMissingLocaleMessages(
    mergeMissingLocaleMessages(base, modular),
    additions,
  ),
  mergeAdditions,
)
