import { useState, useEffect, useRef, useCallback } from 'react'
import { api } from '../api/client'
import { useToast } from './Toast'

interface Props {
  visible: boolean
  onClose: () => void
}

type QrStatus = 'waiting' | 'scanned' | 'scaned_but_redirect' | 'confirmed' | 'expired'

export default function QrModal({ visible, onClose }: Props) {
  const [scanUrl, setScanUrl] = useState('')   // the data to encode as QR code
  const [qrcode, setQrcode] = useState('')      // the hex token for status polling
  const [status, setStatus] = useState<QrStatus>('waiting')
  const [loading, setLoading] = useState(false)
  const [qrDataUrl, setQrDataUrl] = useState('') // generated QR code canvas data URL
  const refreshCountRef = useRef(0)
  const intervalRef = useRef<ReturnType<typeof setInterval>>(undefined)
  const { toast } = useToast()

  // generate QR code from the scannable URL using the canvas API
  const generateQR = useCallback(async (text: string) => {
    try {
      const QRCode = (await import('qrcode')).default
      const dataUrl = await QRCode.toDataURL(text, {
        width: 320,
        margin: 2,
        color: { dark: '#111827', light: '#ffffff' },
      })
      setQrDataUrl(dataUrl)
    } catch {
      // fallback: if QR library fails, just show the URL as text
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
        // generate QR code image from the scannable URL
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
            // 自动刷新二维码
            await fetchQr()
          }
        }
      } catch {
        clearInterval(intervalRef.current)
      }
    }, 1500)

    return () => clearInterval(intervalRef.current)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [visible, qrcode, status, toast, fetchQr])

  if (!visible) return null

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <button className="modal-close" onClick={onClose} style={{
          position: 'absolute',
          top: '16px',
          right: '16px',
          width: '32px',
          height: '32px',
          borderRadius: 'var(--radius-sm)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: 'var(--text-muted)',
          transition: 'background var(--transition)',
          background: 'var(--bg-primary)',
          border: '1px solid var(--border)',
        }}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>

        <h2 style={{ fontSize: '20px', fontWeight: 700, marginBottom: '8px' }}>扫码绑定微信</h2>
        <p style={{ color: 'var(--text-secondary)', fontSize: '14px', marginBottom: '24px' }}>
          请使用微信扫描下方二维码以绑定 ClawBot 账号
        </p>

        <div className="qr-box" style={{
          width: '100%',
          maxWidth: '320px',
          margin: '0 auto',
          borderRadius: 'var(--radius-lg)',
          background: '#ffffff',
          border: '1px solid var(--border)',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          padding: '24px',
          gap: '16px',
        }}>
          {loading && (
            <div style={{ textAlign: 'center', color: 'var(--text-muted)', padding: '48px 0' }}>
              <span className="spinner" style={{ margin: '0 auto 12px' }} />
              <div style={{ fontSize: '14px' }}>获取二维码中...</div>
            </div>
          )}
          {!loading && qrDataUrl && (
            <img
              src={qrDataUrl}
              alt="微信二维码"
              style={{ width: '280px', height: '280px', borderRadius: 'var(--radius-sm)' }}
            />
          )}
          {!loading && !qrDataUrl && scanUrl && (
            // fallback: show scannable URL as text if QR generation failed
            <div style={{ textAlign: 'center', color: 'var(--text-secondary)', fontSize: '14px', lineHeight: 1.6, padding: '24px' }}>
              <p style={{ marginBottom: '12px' }}>请复制下方链接到微信打开</p>
              <code style={{ fontSize: '12px', wordBreak: 'break-all', background: 'var(--bg-primary)', padding: '8px 12px', borderRadius: 'var(--radius-sm)' }}>
                {scanUrl}
              </code>
            </div>
          )}
          {!loading && !scanUrl && !qrDataUrl && (
            <div style={{ color: 'var(--text-muted)', fontSize: '14px', padding: '24px' }}>
              获取二维码失败
            </div>
          )}
        </div>

        <div className="qr-status" style={{
          marginTop: '16px',
          textAlign: 'center',
          fontSize: '14px',
          fontWeight: 600,
        }}>
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
            <span style={{ color: 'var(--success)' }}>绑定成功 ✓</span>
          )}
          {status === 'expired' && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px', alignItems: 'center' }}>
              <span style={{ color: 'var(--danger)' }}>二维码已过期</span>
              <button className="btn btn-secondary btn-sm" onClick={fetchQr}>
                重新获取
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}