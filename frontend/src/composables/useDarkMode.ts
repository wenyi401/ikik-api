import { onBeforeUnmount, onMounted, ref, type Ref } from 'vue'

export function useDarkMode(): Ref<boolean> {
  const isDarkMode = ref(
    typeof document !== 'undefined' && document.documentElement.classList.contains('dark')
  )
  let observer: MutationObserver | null = null

  onMounted(() => {
    isDarkMode.value = document.documentElement.classList.contains('dark')
    observer = new MutationObserver(() => {
      isDarkMode.value = document.documentElement.classList.contains('dark')
    })
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class']
    })
  })

  onBeforeUnmount(() => {
    observer?.disconnect()
    observer = null
  })

  return isDarkMode
}
