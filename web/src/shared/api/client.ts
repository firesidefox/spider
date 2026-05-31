import { authHeaders } from '@/api/auth'

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
    this.name = 'ApiError'
  }
}

interface RequestOptions {
  headers?: Record<string, string>
  responseType?: 'json' | 'text' | 'blob' | 'void'
}

class ApiClient {
  private baseURL = '/api/v1'

  async get<T>(path: string, options?: RequestOptions): Promise<T> {
    return this.request<T>('GET', path, undefined, options)
  }

  async post<T>(path: string, body?: any, options?: RequestOptions): Promise<T> {
    return this.request<T>('POST', path, body, options)
  }

  async patch<T>(path: string, body?: any, options?: RequestOptions): Promise<T> {
    return this.request<T>('PATCH', path, body, options)
  }

  async delete<T>(path: string, body?: any, options?: RequestOptions): Promise<T> {
    return this.request<T>('DELETE', path, body, options)
  }

  async put<T>(path: string, body?: any, options?: RequestOptions): Promise<T> {
    return this.request<T>('PUT', path, body, options)
  }

  async upload<T>(path: string, formData: FormData): Promise<T> {
    return this.request<T>('POST', path, formData)
  }

  async download(path: string): Promise<Blob> {
    return this.request<Blob>('GET', path, undefined, { responseType: 'blob' })
  }

  private async request<T>(
    method: string,
    path: string,
    body?: any,
    options?: RequestOptions
  ): Promise<T> {
    const headers: Record<string, string> = { ...authHeaders() }

    // Auto-detect body type
    let requestBody: any = body
    if (body && !(body instanceof FormData)) {
      headers['Content-Type'] = 'application/json'
      requestBody = JSON.stringify(body)
    }
    // FormData sets its own Content-Type with boundary

    // Merge custom headers
    if (options?.headers) {
      Object.assign(headers, options.headers)
    }

    const res = await fetch(`${this.baseURL}${path}`, {
      method,
      headers,
      body: requestBody,
    })

    if (res.status === 401) {
      // Clear auth state and redirect
      localStorage.removeItem('spider_token')
      window.dispatchEvent(new Event('auth-expired'))
      window.location.href = '/login'
      throw new ApiError(401, 'Unauthorized')
    }

    if (!res.ok) {
      const contentType = res.headers.get('content-type')
      if (contentType?.includes('application/json')) {
        const error = await res.json()
        throw new ApiError(res.status, error.error || 'Request failed')
      }
      throw new ApiError(res.status, `HTTP ${res.status}`)
    }

    // Handle response based on type
    const responseType = options?.responseType || 'json'
    switch (responseType) {
      case 'void':
        return undefined as T
      case 'text':
        return (await res.text()) as T
      case 'blob':
        return (await res.blob()) as T
      case 'json':
      default:
        return res.json()
    }
  }
}

export const api = new ApiClient()
