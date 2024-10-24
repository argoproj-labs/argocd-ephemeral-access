#!/usr/bin/env node

// bin script to run the generator given an input location,
// and copy the generated client code to the output location.
const orval = require('orval').generate;
const fs = require('fs-extra');
const path = require('path');

// TODO: @tanderson9 this is hacky!!
// use a proper cli framework, not environment variables!!
if (!process.env.SPEC_LOCATION)
  throw new Error('SPEC_LOCATION environment variable is not set');
if (!process.env.OUTPUT_LOCATION)
  throw new Error('OUTPUT_LOCATION environment variable is not set');

const orvalConfig = path.join(__dirname, '/orval.config.ts');
const clientFolder = path.join(__dirname, '/generated-client');
// run orval generator by passing in our config.
// the config uses process.env.SPEC_LOCATION as the input location.
orval(orvalConfig).then(() => {
  // copy the `./generated-client` folder to the `OUTPUT_LOCATION` folder
  const outputLocation = process.env.OUTPUT_LOCATION;
  console.log('copying generated client.');
  fs.copySync(clientFolder, outputLocation, {
    filter: (src, dest) => {
      // don't copy node_modules or dist/ folder
      return !src.endsWith('node_modules') && !src.endsWith('dist');
    },
  });
  console.log('copied generated client to ', outputLocation);
});