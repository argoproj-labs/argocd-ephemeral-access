import {History} from 'history';
import * as React from 'react';


export interface ContextApis {
  baseHref: string;
}
export const Context = React.createContext<ContextApis & {history: History}>(null);
export const {Provider, Consumer} = Context;
