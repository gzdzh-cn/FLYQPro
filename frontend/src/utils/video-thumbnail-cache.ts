export interface VideoThumbnailIdentity {
  source: string
  deviceId?: string
  sharedFolderId: string
  entryId?: string
  relativePath: string
  fileSize: number
  modifiedAt?: string
}

interface VideoThumbnailRecord {
  key: string
  dataURL: string
  size: number
  updatedAt: number
}

const databaseName = 'flyqpro-media-cache'
const databaseVersion = 1
const storeName = 'video-thumbnails'
const maxAge = 30 * 24 * 60 * 60 * 1000
const maxRecords = 160
const maxBytes = 64 * 1024 * 1024
const memoryCache = new Map<string, string>()
const pendingLoads = new Map<string, Promise<string>>()
let databasePromise: Promise<IDBDatabase | undefined> | undefined

function cacheKey(identity: VideoThumbnailIdentity) {
  return JSON.stringify([
    identity.source,
    identity.deviceId || '',
    identity.sharedFolderId,
    identity.entryId || '',
    identity.relativePath,
    identity.fileSize,
    identity.modifiedAt || '',
  ])
}

function openDatabase() {
  if (databasePromise) return databasePromise
  if (typeof indexedDB === 'undefined') return Promise.resolve(undefined)
  databasePromise = new Promise((resolve) => {
    const request = indexedDB.open(databaseName, databaseVersion)
    request.onupgradeneeded = () => {
      if (!request.result.objectStoreNames.contains(storeName)) request.result.createObjectStore(storeName, { keyPath: 'key' })
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => resolve(undefined)
  })
  return databasePromise
}

function readRecord(db: IDBDatabase, key: string) {
  return new Promise<VideoThumbnailRecord | undefined>((resolve) => {
    const request = db.transaction(storeName, 'readonly').objectStore(storeName).get(key)
    request.onsuccess = () => resolve(request.result as VideoThumbnailRecord | undefined)
    request.onerror = () => resolve(undefined)
  })
}

async function removeRecord(db: IDBDatabase, key: string) {
  await new Promise<void>((resolve) => {
    const request = db.transaction(storeName, 'readwrite').objectStore(storeName).delete(key)
    request.onsuccess = request.onerror = () => resolve()
  })
}

async function pruneDatabase(db: IDBDatabase) {
  const records = await new Promise<VideoThumbnailRecord[]>((resolve) => {
    const request = db.transaction(storeName, 'readonly').objectStore(storeName).getAll()
    request.onsuccess = () => resolve((request.result || []) as VideoThumbnailRecord[])
    request.onerror = () => resolve([])
  })
  const now = Date.now()
  const valid = records.filter((record) => record.dataURL && now - record.updatedAt <= maxAge)
  valid.sort((left, right) => right.updatedAt - left.updatedAt)
  let total = 0
  const keep = valid.filter((record) => {
    if (valid.indexOf(record) >= maxRecords || total + record.size > maxBytes) return false
    total += record.size
    return true
  })
  const keepKeys = new Set(keep.map((record) => record.key))
  await new Promise<void>((resolve) => {
    const transaction = db.transaction(storeName, 'readwrite')
    const store = transaction.objectStore(storeName)
    records.forEach((record) => { if (!keepKeys.has(record.key)) store.delete(record.key) })
    transaction.oncomplete = transaction.onerror = transaction.onabort = () => resolve()
  })
}

export async function getVideoThumbnail(identity: VideoThumbnailIdentity) {
  const key = cacheKey(identity)
  const memory = memoryCache.get(key)
  if (memory) return memory
  const db = await openDatabase()
  if (!db) return ''
  const record = await readRecord(db, key)
  if (!record || !record.dataURL || Date.now() - record.updatedAt > maxAge) {
    if (record) await removeRecord(db, key)
    return ''
  }
  memoryCache.set(key, record.dataURL)
  return record.dataURL
}

export async function putVideoThumbnail(identity: VideoThumbnailIdentity, dataURL: string) {
  if (!dataURL) return
  const key = cacheKey(identity)
  memoryCache.set(key, dataURL)
  const db = await openDatabase()
  if (!db) return
  const record: VideoThumbnailRecord = { key, dataURL, size: dataURL.length, updatedAt: Date.now() }
  await new Promise<void>((resolve) => {
    const request = db.transaction(storeName, 'readwrite').objectStore(storeName).put(record)
    request.onsuccess = request.onerror = () => resolve()
  })
  await pruneDatabase(db)
}

export async function getOrLoadVideoThumbnail(identity: VideoThumbnailIdentity, loader: () => Promise<string>) {
  const key = cacheKey(identity)
  const existing = pendingLoads.get(key)
  if (existing) return existing
  const request = (async () => {
    const cached = await getVideoThumbnail(identity)
    if (cached) return cached
    const loaded = await loader()
    if (loaded) await putVideoThumbnail(identity, loaded)
    return loaded
  })().catch(() => '')
  pendingLoads.set(key, request)
  try {
    return await request
  } finally {
    if (pendingLoads.get(key) === request) pendingLoads.delete(key)
  }
}

export function captureVideoFrame(url: string): Promise<string> {
  return new Promise((resolve) => {
    const video = document.createElement('video')
    const canvas = document.createElement('canvas')
    let settled = false
    let frameCaptured = false
    const timer = window.setTimeout(() => finish(''), 12000)
    const finish = (value: string) => {
      if (settled) return
      settled = true
      window.clearTimeout(timer)
      video.pause()
      video.removeAttribute('src')
      video.load()
      resolve(value)
    }
    video.crossOrigin = 'anonymous'
    video.muted = true
    video.playsInline = true
    video.preload = 'auto'
    video.addEventListener('error', () => finish(''), { once: true })
    const capture = () => {
      if (settled || frameCaptured || video.readyState < HTMLMediaElement.HAVE_CURRENT_DATA) return
      const width = video.videoWidth
      const height = video.videoHeight
      if (!width || !height) return
      const ratio = Math.min(1, 640 / Math.max(width, height))
      canvas.width = Math.max(1, Math.round(width * ratio))
      canvas.height = Math.max(1, Math.round(height * ratio))
      try {
        const context = canvas.getContext('2d')
        if (!context) return finish('')
        context.drawImage(video, 0, 0, canvas.width, canvas.height)
        frameCaptured = true
        finish(canvas.toDataURL('image/jpeg', 0.8))
      } catch {
        finish('')
      }
    }
    video.addEventListener('loadedmetadata', () => {
      const duration = Number.isFinite(video.duration) ? video.duration : 0
      video.currentTime = duration > 0.4 ? Math.min(duration * 0.1, 1) : 0
    }, { once: true })
    video.addEventListener('loadeddata', capture)
    video.addEventListener('seeked', capture)
    video.src = url
    video.load()
  })
}
