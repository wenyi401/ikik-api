export type AdminComplianceCopyKey =
  | 'title'
  | 'blockingNotice'
  | 'riskNotice'
  | 'version'
  | 'openDocument'
  | 'documentSource'
  | 'inputLabel'
  | 'inputPlaceholder'
  | 'inputMismatch'
  | 'legalNote'
  | 'logout'
  | 'accept'
  | 'accepted'
  | 'acceptFailed'

type ComplianceCopy = Record<AdminComplianceCopyKey, string>

const fallbackCopy: Record<'en' | 'zh', ComplianceCopy> = {
  zh: {
    title: '管理员合规确认',
    blockingNotice: '完成确认后即可继续使用管理员控制台',
    riskNotice: '请阅读下方承诺，并按页面提示完成确认',
    version: '承诺版本',
    openDocument: '查看完整承诺',
    documentSource: '承诺正文来自当前项目仓库；版本更新后需要重新确认',
    inputLabel: '输入下方确认内容',
    inputPlaceholder: '请输入完整确认内容',
    inputMismatch: '确认内容不匹配，请逐字输入上方内容',
    legalNote: '提交后，系统会记录当前管理员、确认版本、时间及必要的安全审计信息',
    logout: '退出登录',
    accept: '确认并继续',
    accepted: '合规确认已记录',
    acceptFailed: '提交确认失败',
  },
  en: {
    title: 'Administrator compliance acknowledgment',
    blockingNotice: 'Complete this acknowledgment to continue using the admin console',
    riskNotice: 'Read the commitment below, then complete the requested confirmation',
    version: 'Commitment version',
    openDocument: 'View full commitment',
    documentSource: 'The commitment is stored in this project repository; a new version requires acknowledgment again',
    inputLabel: 'Enter the confirmation shown below',
    inputPlaceholder: 'Enter the complete confirmation',
    inputMismatch: 'The confirmation does not match. Enter the text above exactly',
    legalNote: 'The system records the administrator, version, time, and required security audit information',
    logout: 'Log out',
    accept: 'Acknowledge and continue',
    accepted: 'Compliance acknowledgment recorded',
    acceptFailed: 'Failed to submit acknowledgment',
  },
}

export function resolveAdminComplianceCopy(
  locale: string,
  key: AdminComplianceCopyKey,
  translated?: string,
): string {
  const path = `adminCompliance.${key}`
  if (translated && translated !== path) {
    return translated
  }

  const language = locale.toLowerCase().startsWith('zh') ? 'zh' : 'en'
  return fallbackCopy[language][key]
}
