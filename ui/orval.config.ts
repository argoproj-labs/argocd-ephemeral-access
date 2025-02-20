module.exports = {
  main: {
    input: "./src/gen/schema.yaml",
    output: {
      target: "./src/gen/ephemeral-access-api.ts",
      prettier: true,
      baseUrl: '/extensions/ephemeral/',
    },
  },
};