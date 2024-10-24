import { defineConfig } from 'orval';
const path = require('path');

// spec is resolved relative to the working directory of the running process,
// not relative to this file!
const specLocation = path.resolve('.', 'openapi/ephemeral_openapi.json');

const mutatorLocation = path.resolve(
  __dirname,
  './src/gen/transformers/axiosInstance.ts'
);

export default defineConfig({
  'ephemeral-access': {
    output: {
      mode: 'single',
      workspace: './src/gen',
      target: './',

      client: 'react-query',
      mock: true,
      clean: true,
      prettier: true,
    override: {
      mutator: {
        path: mutatorLocation,
        name: 'injectAxios',
      },
    },
  },

  input: {
    target: specLocation,
  },
},
});