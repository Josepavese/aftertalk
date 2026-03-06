import { readFileSync } from 'fs';
import { parse } from 'yaml';
export class ConfigLoader {
    configPath;
    config = null;
    constructor(configPath = './config.yaml') {
        this.configPath = configPath;
    }
    load() {
        if (this.config)
            return this.config;
        const content = readFileSync(this.configPath, 'utf8');
        this.config = parse(content);
        return this.config;
    }
}
//# sourceMappingURL=config.js.map