import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';
import commandSidebar from './command-sidebar.json';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

/**
 * Creating a sidebar enables you to:
 - create an ordered group of docs
 - render a sidebar for each doc of that group
 - provide next/previous navigation

 The sidebars can be generated from the filesystem, or explicitly defined here.

 Create as many sidebars as you want.
 */
const sidebars: SidebarsConfig = {
  tutorialSidebar: [
    'intro',
    'getting-started',
    {
      type: 'category',
      label: 'Features',
      collapsed: false,
      items: [
        'app-configuration',
        'languages',
        'services',
        'traffic-routing',
        'scaling',
        'disks',
        'tls',
        'firewall',
        'route-oidc',
        'ci-deploy',
        'admin-interface',
        'working-with-miren-cloud',
        'observability',
        'logs',
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      collapsed: false,
      items: [
        'system-requirements',
        'app-toml',
        'server-config',
        {
          type: 'category',
          label: 'CLI',
          collapsed: true,
          link: {
            type: 'doc',
            id: 'commands',
          },
          items: commandSidebar as any[],
        },
      ],
    },
    {
      type: 'category',
      label: 'Resources',
      collapsed: false,
      items: [
        'troubleshooting',
        'terminology',
        'labs',
        'changelog',
        'cloud-updates',
        'conduct',
        {
          type: 'link',
          label: 'How Miren Compares',
          href: 'https://miren.dev/compare',
        },
      ],
    },
  ],
};

export default sidebars;
