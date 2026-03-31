// @ts-check

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'Stroppy Cloud',
  tagline: 'Database Testing Orchestrator',
  favicon: 'img/favicon.ico',
  url: 'https://stroppy-io.github.io',
  baseUrl: '/stroppy-cloud/',
  organizationName: 'stroppy-io',
  projectName: 'stroppy-cloud',
  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'warn',

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          sidebarPath: require.resolve('./sidebars.js'),
          editUrl: 'https://github.com/stroppy-io/stroppy-cloud/tree/main/docs/',
        },
        theme: {
          customCss: [],
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      navbar: {
        title: 'Stroppy Cloud',
        items: [
          {
            type: 'docSidebar',
            sidebarId: 'docsSidebar',
            position: 'left',
            label: 'Docs',
          },
          {
            href: 'https://github.com/stroppy-io/stroppy-cloud',
            label: 'GitHub',
            position: 'right',
          },
        ],
      },
      footer: {
        style: 'dark',
        links: [
          {
            title: 'Docs',
            items: [
              { label: 'Getting Started', to: '/docs/intro' },
              { label: 'Architecture', to: '/docs/architecture/overview' },
              { label: 'API Reference', to: '/docs/api/external' },
            ],
          },
          {
            title: 'More',
            items: [
              {
                label: 'GitHub',
                href: 'https://github.com/stroppy-io/stroppy-cloud',
              },
            ],
          },
        ],
        copyright: `Copyright ${new Date().getFullYear()} Stroppy IO.`,
      },
    }),
};

module.exports = config;
