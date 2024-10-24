import Axios, { AxiosRequestConfig } from 'axios';

/**
 * the axios instance that will be used to make the requests.
 * its config can be modified at runtime.
 */
export const AXIOS_INSTANCE = Axios.create({ baseURL: '/extensions/ephemeral/' });

/**
 * whenever we need to make a request, this function takes an axios config object
 * and applies it to our shared axios instance.
 * this lets us make changes to the shared instance, and have them applied
 * whenever we make a request.
 */
export const injectAxios = <T>(config: AxiosRequestConfig): Promise<T> => {
  const source = Axios.CancelToken.source();
  const promise = AXIOS_INSTANCE({ ...config, cancelToken: source.token }).then(
    ({ data }) => data
  );

  // @ts-ignore
  promise.cancel = () => {
    source.cancel('Query was cancelled by React Query');
  };

  return promise;
};

export default injectAxios;