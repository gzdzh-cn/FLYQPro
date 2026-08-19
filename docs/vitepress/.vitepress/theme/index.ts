import DefaultTheme from 'vitepress/theme'
import { onContentUpdated } from 'vitepress'
import type { Theme } from 'vitepress'
import './custom.css'

const IMAGE_SELECTOR = '.vp-doc img:not([data-no-zoom])'

let zoomOverlay: HTMLDivElement | null = null
let zoomPreview: HTMLImageElement | null = null
let activeImage: HTMLImageElement | null = null

function closeImage() {
  if (!zoomOverlay || !zoomPreview) return

  zoomOverlay.hidden = true
  zoomOverlay.classList.remove('is-open')
  zoomPreview.removeAttribute('src')
  document.body.classList.remove('image-zoom-open')
  activeImage?.removeAttribute('aria-expanded')
  activeImage = null
}

function openImage(image: HTMLImageElement) {
  if (!zoomOverlay || !zoomPreview) return

  activeImage?.removeAttribute('aria-expanded')
  activeImage = image
  zoomPreview.src = image.currentSrc || image.src
  zoomPreview.alt = image.alt
  zoomOverlay.hidden = false
  zoomOverlay.classList.add('is-open')
  document.body.classList.add('image-zoom-open')
  image.setAttribute('aria-expanded', 'true')
}

function createZoomOverlay() {
  if (zoomOverlay && zoomPreview) return

  zoomOverlay = document.createElement('div')
  zoomOverlay.className = 'image-zoom-overlay'
  zoomOverlay.hidden = true
  zoomOverlay.setAttribute('role', 'dialog')
  zoomOverlay.setAttribute('aria-modal', 'true')
  zoomOverlay.setAttribute('aria-label', '图片预览')

  zoomPreview = document.createElement('img')
  zoomPreview.className = 'image-zoom-preview'
  zoomPreview.alt = ''
  zoomOverlay.appendChild(zoomPreview)
  zoomOverlay.addEventListener('click', closeImage)
  document.body.appendChild(zoomOverlay)
}

function enhanceImages() {
  document.querySelectorAll<HTMLImageElement>(IMAGE_SELECTOR).forEach((image) => {
    image.classList.add('vp-zoomable-image')
    image.tabIndex = 0
    image.setAttribute('role', 'button')
    image.setAttribute('aria-label', `${image.alt || '图片'}，点击放大`)
  })
}

function setupImageZoom() {
  createZoomOverlay()
  enhanceImages()

  document.addEventListener('click', (event) => {
    const target = event.target
    if (!(target instanceof HTMLImageElement) || !target.matches(IMAGE_SELECTOR)) return

    event.preventDefault()
    openImage(target)
  })

  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && !zoomOverlay?.hidden) closeImage()

    const target = event.target
    if (
      (event.key === 'Enter' || event.key === ' ') &&
      target instanceof HTMLImageElement &&
      target.matches(IMAGE_SELECTOR)
    ) {
      event.preventDefault()
      openImage(target)
    }
  })
}

export default {
  extends: DefaultTheme,
  enhanceApp({ router }) {
    if (typeof window === 'undefined') return

    setupImageZoom()
    onContentUpdated(enhanceImages)
    router.onAfterRouteChanged = () => {
      closeImage()
      enhanceImages()
    }
  },
} satisfies Theme
