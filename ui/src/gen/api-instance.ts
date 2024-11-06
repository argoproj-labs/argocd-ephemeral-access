import axios, { AxiosError, AxiosRequestConfig } from "axios";

export const apiInstance = axios.create({
  baseURL: "/extensions/ephemeral/",
  headers: {
    "Content-Type": "application/json",
  },
  withCredentials: true,
});

export const createInstance = <T>(
  config: AxiosRequestConfig,
  options?: AxiosRequestConfig
): Promise<T> => {
  return apiInstance({
    ...config,
    ...options,
  }).then((res) => res.data);
};

export type BodyType<Data> = Data;

export type ErrorType<Error> = AxiosError<Error>;