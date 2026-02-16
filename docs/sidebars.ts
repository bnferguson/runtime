import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

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
        'languages',
        'services',
        'scaling',
        'disks',
        'firewall',
        'route-oidc',
        'admin-interface',
        'working-with-miren-cloud',
        'observability',
      ],
    },
    {
      type: 'category',
      label: 'CLI Reference',
      collapsed: false,
      items: [
        'cli-reference',
        'cli/app',
        'cli/logs',
        'cli/sandbox',
        'cli/disk',
        'cli/entity',
        'cli/admin',
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
