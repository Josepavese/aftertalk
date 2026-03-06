import { readFileSync } from 'fs';
import { parse } from 'yaml';
import { SDKConfig } from './types.js';

export class ConfigLoader {
  private config: SDKConfig | null = null;

  constructor(private configPath: string = './config.yaml') {}

  load(): SDKConfig {
    if (this.config) return this.config;
    const content = readFileSync(this.configPath, 'utf8');
    this.config = parse(content) as SDKConfig;
    return this.config;
  }
}
