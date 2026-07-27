import { useState, useEffect, useRef, useCallback } from 'react'
import { api } from '../api/client'
import { useToast } from './Toast'
import Button from './ui/Button'
import Modal from './ui/Modal'

interface Props {
  visible: boolean
  onClose: () => void
}

type QrStatus = 'waiting' | 'scanned' | 'scaned_but_redirect' | 'confirmed' | 'expired'

export default function QrModal({ visible, onClose }: Props) {
  const [scanUrl, setScanUrl] = useState('')
  const [qrcode, setQrcode] = useState('')
  const [status, setStatus] = useState<QrStatus>('waiting')
  const [loading, setLoading] = useState(false)
  const [qrDataUrl, setQrDataUrl] = useState('')
  const refreshCountRef = useRef(0)
  const intervalRef = useRef<ReturnType<typeof setInterval>>(undefined)
  const { toast } = useToast()

  const generateQR = useCallback(async (text: string) => {
    try {
      const QRCode = (await import('qrcode')).default
      const style = getComputedStyle(document.documentElement)
      const darkColor = style.getPropertyValue('--n-900').trim() || ''
      const lightColor = style.getPropertyValue('--n-100').trim() || ''
      const dataUrl = await QRCode.toDataURL(text, {
        width: 320,
        margin: 2,
        color: { dark: darkColor, light: lightColor },
      })
      setQrDataUrl(dataUrl)
    } catch {
      setQrDataUrl('')
    }
  }, [])

  const fetchQr = useCallback(async () => {
    setLoading(true)
    setStatus('waiting')
    setScanUrl('')
    setQrcode('')
    setQrDataUrl('')
    try {
      const res = await api.post<{ success: boolean; data: { qrcode_url: string; qrcode: string } }>('/api/v1/wechat/qrcode')
      if (res.data?.qrcode_url) {
        setScanUrl(res.data.qrcode_url)
        setQrcode(res.data.qrcode)
        setStatus('waiting')
        await generateQR(res.data.qrcode_url)
      } else {
        toast('二维码数据异常', 'error')
      }
    } catch {
      toast('获取二维码失败', 'error')
    } finally {
      setLoading(false)
    }
  }, [toast, generateQR])

  // Reset state when closed; fetch QR code when opened.
  // Modal handles focus trap, Escape key, backdrop click, and body overflow.
  useEffect(() => {
    if (!visible) {
      clearInterval(intervalRef.current)
      setScanUrl('')
      setQrcode('')
      setStatus('waiting')
      setQrDataUrl('')
      refreshCountRef.current = 0
      return
    }
    fetchQr()
  }, [visible, fetchQr])

  // Poll for QR scan status
  useEffect(() => {
    if (!visible || !qrcode || status === 'confirmed' || status === 'expired') {
      clearInterval(intervalRef.current)
      return
    }

    intervalRef.current = setInterval(async () => {
      try {
        const res = await api.post<{ success: boolean; data: { status: string; redirect_host?: string } }>('/api/v1/wechat/qrcode/status', { qrcode })
        const st = res.data?.status
        if (st === 'scanned') {
          setStatus('scanned')
        } else if (st === 'scaned_but_redirect') {
          setStatus('scaned_but_redirect')
        } else if (st === 'confirmed') {
          setStatus('confirmed')
          toast('微信账号绑定成功', 'success')
          clearInterval(intervalRef.current)
          onClose()
        } else if (st === 'expired') {
          refreshCountRef.current++
          if (refreshCountRef.current > 3) {
            setStatus('expired')
            clearInterval(intervalRef.current)
          } else {
            await fetchQr()
          }
        }
      } catch {
        clearInterval(intervalRef.current)
      }
    }, 1500)

    return () => clearInterval(intervalRef.current)
  }, [visible, qrcode, status, toast, fetchQr, onClose])

  return (
    <Modal open={visible} onClose={onClose} title="扫码绑定微信" maxWidth="400px">
      <p style={{ color: 'var(--text-secondary)', fontSize: 'var(--text-sm)', marginBottom: 'var(--space-6)' }}>
        请使用微信扫描下方二维码以绑定 ClawBot 账号
      </p>

      <div className="qr-code-box">
        {loading && (
          <div style={{ textAlign: 'center', color: 'var(--text-muted)', padding: 'var(--space-12) 0' }}>
            <span className="spinner" style={{ margin: '0 auto var(--space-3)' }} />
            <div style={{ fontSize: 'var(--text-sm)' }}>获取二维码中...</div>
          </div>
        )}
        {!loading && qrDataUrl && (
          <img
            src={qrDataUrl}
            alt="微信二维码"
            className="qr-code-image"
          />
        )}
        {!loading && !qrDataUrl && scanUrl && (
          <div style={{ textAlign: 'center', color: 'var(--text-secondary)', fontSize: 'var(--text-sm)', lineHeight: 1.6, padding: 'var(--space-6)' }}>
            <p style={{ marginBottom: 'var(--space-3)' }}>请复制下方链接到微信打开</p>
            <code className="code-block" style={{ fontSize: 'var(--text-xs)', wordBreak: 'break-all' }}>
              {scanUrl}
            </code>
          </div>
        )}
        {!loading && !scanUrl && !qrDataUrl && (
          <div style={{ color: 'var(--text-muted)', fontSize: 'var(--text-sm)', padding: 'var(--space-6)' }}>
            获取二维码失败
          </div>
        )}
      </div>

      <div className="qr-status" style={{ marginTop: 'var(--space-4)', textAlign: 'center', fontSize: 'var(--text-sm)', fontWeight: 600 }}>
        {status === 'waiting' && (
          <span style={{ color: 'var(--accent-hover)' }}>等待扫码...</span>
        )}
        {status === 'scanned' && (
          <span style={{ color: 'var(--warning)' }}>已扫码，请在手机上确认</span>
        )}
        {status === 'scaned_but_redirect' && (
          <span style={{ color: 'var(--warning)' }}>扫码已确认，正在重定向...</span>
        )}
        {status === 'confirmed' && (
          <span style={{ color: 'var(--success)' }}>绑定成功</span>
        )}
        {status === 'expired' && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)', alignItems: 'center' }}>
            <span style={{ color: 'var(--danger)' }}>二维码已过期</span>
            <Button variant="secondary" size="sm" onClick={fetchQr}>
              重新获取
            </Button>
          </div>
        )}
      </div>
    </Modal>
  )
}