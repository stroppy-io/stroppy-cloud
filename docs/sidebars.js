/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  docsSidebar: [
    'intro',
    {
      type: 'category',
      label: 'Getting Started',
      items: [
        'getting-started/installation',
        'getting-started/quick-start',
        'getting-started/docker',
      ],
    },
    {
      type: 'category',
      label: 'Architecture',
      items: [
        'architecture/overview',
        'architecture/dag-engine',
        'architecture/agent-protocol',
      ],
    },
    {
      type: 'category',
      label: 'Databases',
      items: [
        'databases/postgres',
        'databases/mysql',
        'databases/picodata',
        'databases/topologies',
      ],
    },
    {
      type: 'category',
      label: 'API Reference',
      items: [
        'api/external',
        'api/agent',
        'api/admin',
        'api/websocket',
        'api/metrics',
      ],
    },
    {
      type: 'category',
      label: 'Configuration',
      items: [
        'configuration/run-config',
        'configuration/server-settings',
        'configuration/packages',
        'configuration/cloud-init',
      ],
    },
    {
      type: 'category',
      label: 'Development',
      items: [
        'development/building',
        'development/testing',
        'development/contributing',
      ],
    },
  ],
};

module.exports = sidebars;
