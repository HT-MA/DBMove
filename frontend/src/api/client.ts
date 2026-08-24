import axios from 'axios';

export class ApiError extends Error {
  code?: string;
  constructor(code: string | undefined, message: string) {
    super(message);
    this.code = code;
  }
}

interface ApiEnvelope<T> {
  success: boolean;
  data?: T;
  error?: { code: string; message: string };
}

const http = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
});

http.interceptors.response.use(
  (response) => {
    const body = response.data as ApiEnvelope<unknown>;
    if (body && typeof body === 'object' && 'success' in body) {
      if (!body.success) {
        return Promise.reject(
          new ApiError(body.error?.code, body.error?.message || 'Request failed')
        );
      }
      response.data = body.data;
    }
    return response;
  },
  (error) => {
    const msg =
      error?.response?.data?.error?.message ||
      error?.message ||
      'Network error';
    return Promise.reject(new ApiError(error?.response?.data?.error?.code, msg));
  }
);

export async function apiGet<T>(url: string, params?: Record<string, unknown>): Promise<T> {
  const resp = await http.get<T>(url, { params });
  return resp.data;
}

export async function apiPost<T>(url: string, body?: unknown): Promise<T> {
  const resp = await http.post<T>(url, body);
  return resp.data;
}

export async function apiPut<T>(url: string, body?: unknown): Promise<T> {
  const resp = await http.put<T>(url, body);
  return resp.data;
}

export async function apiDelete<T>(url: string): Promise<T> {
  const resp = await http.delete<T>(url);
  return resp.data;
}

export default http;
