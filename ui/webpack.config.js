const path = require("path");
const webpack = require("webpack");
const TerserWebpackPlugin = require("terser-webpack-plugin");

const extName = "ephemeral-access";

const config = {
  entry: {
    extension: "./src/index.tsx",
  },
  output: {
    filename: `extensions-${extName}.js`,
    path: path.resolve(__dirname, `dist/resources/extension-${extName}.js`),
    libraryTarget: "window",
    library: ["tmp", "extensions"],
  },
  resolve: {
    extensions: [".ts", ".tsx", ".js", ".json", ".ttf"],
  },
  externals: {
    react: "React",
    "react-dom": "ReactDOM",
    moment: "Moment",
  },
  optimization: {
    minimize: true,
    minimizer: [
      new TerserWebpackPlugin({
        terserOptions: {
          format: {
            comments: false,
          },
        },
        extractComments: false,
      }),
    ],
  },
  plugins: [
    new webpack.DefinePlugin({
      SYSTEM_INFO: JSON.stringify({
        version: process.env.VERSION || 'latest',
      }),
    }),
  ],
  module: {
    rules: [
      {
        test: /\.(ts|js)x?$/,
        loader: "esbuild-loader",
        options: {
          loader: "tsx",
          target: "es2015",
        },
      },
      {
        test: /\.scss$/,
        use: ['style-loader', 'raw-loader', 'sass-loader'],
      },
      {
        test: /\.css$/,
        use: ['style-loader', 'raw-loader'],
      },

    ],
  },
};

module.exports = config;