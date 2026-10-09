module.exports = {
  configureWebpack: {
    performance: {
      hints: false,
    },
    devServer: {
      historyApiFallback: true,
      proxy: {
        '^/api': {
          target: 'http://127.0.0.1:3000',
        },
      },
    },
  },
  chainWebpack: (config) => {
    if (process.env.NODE_ENV === 'test') {
      // In test mode babel transpiles modules to CommonJS, while vuetify-loader
      // prepends ES `import` statements to compiled templates. Mixing both makes
      // webpack drop the template's `render` export, so every SFC fails to mount
      // in unit tests. Components are registered by `Vue.use(Vuetify)` instead.
      config.plugins.delete('VuetifyLoaderPlugin');
    }

    config.plugin('html')
      .tap((args) => {
        // eslint-disable-next-line no-param-reassign
        args[0].minify = false;
        return args;
      });
  },
  transpileDependencies: [
    'vuetify',
  ],
  publicPath: './',
  outputDir: '../api/public',
};
