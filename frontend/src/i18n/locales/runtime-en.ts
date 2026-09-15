import base from './en'
import modular from './en/index'
import additions from './runtime-additions-en'
import mergeAdditions from './merge-additions-en'
import { mergeMissingLocaleMessages } from './merge'

export default mergeMissingLocaleMessages(
  mergeMissingLocaleMessages(
    mergeMissingLocaleMessages(base, modular),
    additions,
  ),
  mergeAdditions,
)
