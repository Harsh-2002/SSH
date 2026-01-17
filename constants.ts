import { ToolCategory, ConfigItem, InstallMethod, NavItem } from './types';

export const NAV_ITEMS: NavItem[] = [
  { label: 'Start', href: '#quickstart' },
  { label: 'How it Works', href: '#how-it-works' },
  { label: 'Screenshots', href: '#screenshots' },
  { label: 'Tools', href: '#tools' },
  { label: 'Config', href: '#configuration' },
];

export const INSTALL_METHODS: InstallMethod[] = [
  {
    id: 'docker',
    label: 'Docker CLI',
    commands: `docker run -d \\
  --name ssh-mcp \\
  -p 8000:8000 \\
  -v ssh-mcp-data:/data \\
  firstfinger/ssh-mcp:latest`,
    note: 'Ensure volume permissions if using bind mounts.'
  },
  {
    id: 'compose',
    label: 'docker-compose.yml',
    commands: `services:
  ssh-mcp:
    image: firstfinger/ssh-mcp:latest
    container_name: ssh-mcp
    ports:
      - "8000:8000"
    volumes:
      - ssh-data:/data
    restart: unless-stopped

volumes:
  ssh-data:`,
    note: 'Save as docker-compose.yml and run: docker compose up -d'
  },
  {
    id: 'local',
    label: 'Local Python',
    commands: `# Install package
pip install .

# Run server
uvicorn ssh.server_all:app --host 0.0.0.0 --port 8000`,
    note: 'Requires Python 3.10+'
  }
];

export const TOOL_CATEGORIES: ToolCategory[] = [
  {
    title: 'Core & System',
    tools: [
      { name: 'connect(host,...)', description: 'Open SSH connection. Alias auto-generated as user@host if not provided.' },
      { name: 'disconnect(alias)', description: 'Close one or all SSH connections.' },
      { name: 'identity()', description: 'Get server\'s public key for authorized_keys.' },
      { name: 'info()', description: 'Get remote OS/kernel/shell information.' },
      { name: 'run(command)', description: 'Execute any shell command.' },
    ]
  },
  {
    title: 'File Operations',
    tools: [
      { name: 'read(path)', description: 'Read remote file content.' },
      { name: 'write(path, content)', description: 'Create/overwrite remote file.' },
      { name: 'edit(path, old, new)', description: 'Safe text replacement in a file.' },
      { name: 'list_dir(path)', description: 'List directory contents.' },
      { name: 'sync(src, dst, ...)', description: 'Stream file directly between two remote nodes.' },
    ]
  },
  {
    title: 'DevOps & Monitoring',
    tools: [
      { name: 'search_files(pattern)', description: 'Find files using POSIX find.' },
      { name: 'search_text(pattern)', description: 'Search in files using grep.' },
      { name: 'package_manage(pkg)', description: 'Install/check packages (apt, apk, dnf, yum).' },
      { name: 'diagnose_system()', description: 'One-click SRE health check (Load, OOM, Disk).' },
      { name: 'journal_read()', description: 'Read system logs (systemd/syslog).' },
      { name: 'docker_ps()', description: 'List Docker containers.' },
    ]
  },
  {
    title: 'Database',
    tools: [
      { name: 'db_query(...)', description: 'Execute SQL/CQL/MongoDB query in container.' },
      { name: 'db_schema(...)', description: 'Get database/collection schema.' },
      { name: 'list_db_containers()', description: 'Find database containers on host.' },
    ]
  }
];

export const CONFIG_ITEMS: ConfigItem[] = [
  { variable: 'PORT', type: 'Integer', default: '8000', description: 'The port the HTTP server listens on.' },
  { variable: 'SSH_MCP_SESSION_HEADER', type: 'String', default: 'X-Session-Key', description: 'Header used for smart session caching.' },
  { variable: 'SSH_MCP_SESSION_TIMEOUT', type: 'Integer', default: '300', description: 'Idle timeout for cached sessions in seconds.' },
  { variable: 'SSH_MCP_GLOBAL_STATE', type: 'Boolean', default: 'false', description: 'If true, a single SSH manager is shared by all clients.' },
  { variable: 'SSH_MCP_COMMAND_TIMEOUT', type: 'Float', default: '120.0', description: 'Maximum time (seconds) allowed for an SSH command.' },
  { variable: 'SSH_MCP_MAX_OUTPUT', type: 'Integer', default: '51200', description: 'Maximum byte size of command output returned.' },
  { variable: 'SSH_MCP_DEBUG_ASYNCSSH', type: 'Boolean', default: 'false', description: 'Enable verbose debug logs for the asyncssh library.' },
];