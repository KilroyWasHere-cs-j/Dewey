<script lang="ts">
	import { fade } from 'svelte/transition';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Topbar from '$lib/components/Topbar.svelte';
	import ScrollToTop from '$lib/components/ScrollToTop.svelte';
	import { settings, ACCENT } from '$lib/stores/settings.svelte';

	let mainClass = $derived(
		settings.value.layoutDensity === 'compact'
			? 'max-w-4xl space-y-4 p-4'
			: 'max-w-4xl space-y-6 p-6'
	);
	let headingClass = $derived(ACCENT[settings.value.accentColor].text);
	let sidebarOpen = $state(settings.value.defaultSidebarOpen);
	const toggleSidebar = () => (sidebarOpen = !sidebarOpen);

	const toc = [
		{ href: '#overview', label: 'Overview' },
		{ href: '#architecture', label: 'Architecture' },
		{ href: '#stack', label: 'Tech Stack' },
		{ href: '#api', label: 'API Reference' },
		{ href: '#cli', label: 'CLI Tool' },
		{ href: '#mcp', label: 'MCP Server' },
		{ href: '#testing', label: 'Testing Tools' },
		{ href: '#access', label: 'Access Control' },
		{ href: '#plugins', label: 'Plugin System' },
		{ href: '#database', label: 'Database' },
		{ href: '#daemon', label: 'Daemon' },
		{ href: '#logging', label: 'Logging' },
		{ href: '#config', label: 'Configuration' },
		{ href: '#deployment', label: 'Deployment' },
		{ href: '#troubleshooting', label: 'Troubleshooting' }
	];
</script>

<svelte:head>
	<title>Developer Docs — Dewey</title>
</svelte:head>

<div class="flex min-h-screen bg-gray-100 dark:bg-gray-900">
	<Sidebar open={sidebarOpen} toggle={toggleSidebar} />

	<div class="flex flex-1 flex-col">
		<Topbar {sidebarOpen} {toggleSidebar} />

		<main id="main-content" class={mainClass}>

			<!-- Header -->
			<div>
				<p class="text-sm text-gray-500 dark:text-gray-400">
					<a href="/docs" class="hover:underline">Documentation</a> / Developer Guide
				</p>
				<h1 class="mt-1 text-2xl font-bold text-gray-800 dark:text-white">Developer Guide</h1>
				<p class="text-sm text-gray-500 dark:text-gray-400">
					Architecture, API, plugins, database, and deployment reference for engineers.
				</p>

				<!-- Table of contents -->
				<div class="mt-4 flex flex-wrap gap-2">
					{#each toc as item}
						<a
							href={item.href}
							class="rounded-full border border-gray-200 bg-white px-3 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-gray-700"
						>
							{item.label}
						</a>
					{/each}
				</div>
			</div>

			<!-- ── Overview ── -->
			<section id="overview" transition:fade={{ duration: 300 }} class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Overview</h2>
				<p class="mb-3 text-sm text-gray-600 dark:text-gray-300">
					Dewey is a backend document management and file sorting service. It exposes a REST API for
					file ingestion, retrieval, and deletion. On upload, each file is validated, scanned for
					barcodes, run through a Lua plugin chain, copied to permanent storage, and recorded in
					a MySQL database alongside its metadata.
				</p>
				<p class="text-sm text-gray-600 dark:text-gray-300">
					The admin portal (this frontend) is a SvelteKit application that provides a real-time
					analytics dashboard, settings management, and this documentation.
				</p>
			</section>

			<!-- ── Architecture ── -->
			<section id="architecture" transition:fade={{ duration: 300 }} class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Architecture</h2>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">File Ingest Pipeline</h3>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					Every uploaded file passes through a staged pipeline before it is permanently stored.
				</p>
				<ol class="mb-6 space-y-2 text-sm text-gray-600 dark:text-gray-300">
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">1</span><span><strong class="text-gray-700 dark:text-gray-200">Receive</strong> — multipart/form-data POST to <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/upload</code>. File and metadata fields are extracted.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">2</span><span><strong class="text-gray-700 dark:text-gray-200">Validate</strong> — extension checked against an allowlist; file header bytes checked for PE (Windows) and ELF (Linux) executable signatures. Both checks must pass.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">3</span><span><strong class="text-gray-700 dark:text-gray-200">Cache</strong> — file is written to <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">./cache</code> with a Unix timestamp prefix to avoid collisions. SHA-256 hash is computed.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">4</span><span><strong class="text-gray-700 dark:text-gray-200">Barcode scan</strong> — the cached file is scanned via gozxing. Result stored in the DB entry.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">5</span><span><strong class="text-gray-700 dark:text-gray-200">OnUpload plugins</strong> — a tag-only notification hook (can't veto the upload), able to mutate metadata such as <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">entry.Meta</code>.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">6</span><span><strong class="text-gray-700 dark:text-gray-200">OnFilter plugins</strong> — all registered OnFilter plugins run in salience order, each able to mutate the file's destination path.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">7</span><span><strong class="text-gray-700 dark:text-gray-200">Store</strong> — file is copied from cache to <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">./store</code> at the path determined by the plugin chain.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">8</span><span><strong class="text-gray-700 dark:text-gray-200">Record</strong> — a row is inserted into the <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">files</code> table and a row into the <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">meta</code> table.</span></li>
				</ol>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">File Retrieval</h3>
				<p class="text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">GET /files/:filename/false</code> —
					checks cache first; falls back to a DB path lookup. Returns the file stream directly.
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">GET /files/:filename/true</code> returns
					the file's metadata record as JSON instead of streaming its bytes.
				</p>
			</section>

			<!-- ── Tech Stack ── -->
			<section id="stack" transition:fade={{ duration: 300 }} class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Tech Stack</h2>
				<div class="grid gap-4 sm:grid-cols-2">
					<div>
						<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Backend</h3>
						<ul class="space-y-2 text-sm text-gray-600 dark:text-gray-300">
							<li><strong class="text-gray-700 dark:text-gray-200">Go</strong> — compiled, statically typed. Goroutines handle concurrency. Fast build and single-binary deployment.</li>
							<li><strong class="text-gray-700 dark:text-gray-200">Gin</strong> — HTTP router and middleware framework.</li>
							<li><strong class="text-gray-700 dark:text-gray-200">MySQL</strong> — primary datastore via <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">database/sql</code> with connection pooling.</li>
							<li><strong class="text-gray-700 dark:text-gray-200">GopherLua</strong> — embedded Lua VM for the plugin system.</li>
							<li><strong class="text-gray-700 dark:text-gray-200">gozxing</strong> — barcode/QR decoding.</li>
							<li><strong class="text-gray-700 dark:text-gray-200">go-gin-prometheus</strong> — Prometheus metrics middleware.</li>
						</ul>
					</div>
					<div>
						<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Frontend</h3>
						<ul class="space-y-2 text-sm text-gray-600 dark:text-gray-300">
							<li><strong class="text-gray-700 dark:text-gray-200">SvelteKit</strong> — file-based routing, SSR-capable, reactive runes (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">$state</code>, <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">$derived</code>).</li>
							<li><strong class="text-gray-700 dark:text-gray-200">Tailwind CSS</strong> — utility-first styling with dark mode support.</li>
							<li><strong class="text-gray-700 dark:text-gray-200">Flowbite-Svelte</strong> — UI component library.</li>
						</ul>

						<h3 class="mb-2 mt-4 text-sm font-semibold text-gray-700 dark:text-gray-200">Infrastructure</h3>
						<ul class="space-y-2 text-sm text-gray-600 dark:text-gray-300">
							<li><strong class="text-gray-700 dark:text-gray-200">Podman</strong> — container runtime (Docker-compatible). All services run in a single Pod.</li>
							<li><strong class="text-gray-700 dark:text-gray-200">Prometheus</strong> — metrics scraping and storage.</li>
						</ul>
					</div>
				</div>
			</section>

			<!-- ── API Reference ── -->
			<section id="api" transition:fade={{ duration: 300 }} class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">API Reference</h2>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					All endpoints are served on port <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">8080</code>.
					Plugin and DB context is injected into every handler via Gin middleware. Every route also
					passes through the <a href="#access" class="underline">Access Control</a> allowlist first.
				</p>

				<div class="overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="border-b border-gray-200 text-left dark:border-gray-700">
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Method</th>
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Path</th>
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Handler</th>
								<th class="pb-2 font-semibold text-gray-700 dark:text-gray-200">Description</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-100 dark:divide-gray-700">
							{#each [
								{ method: 'GET',    path: '/',                                        handler: 'index',                  desc: 'Health check. Returns server status and Unix timestamp.' },
								{ method: 'GET',    path: '/version',                                 handler: 'versionInfo',            desc: "Returns the backend's release version and the git branch it was built from." },
								{ method: 'POST',   path: '/upload',                                  handler: 'uploadFile',             desc: 'Upload a file with metadata fields. Accepts multipart/form-data.' },
								{ method: 'GET',    path: '/files',                                   handler: 'listFiles',              desc: 'List all filenames currently in the upload cache.' },
								{ method: 'GET',    path: '/files/:filename/:meta',                   handler: 'getFile',                desc: 'Retrieve a file by name. Set :meta to false for file stream; true is not yet implemented.' },
								{ method: 'DELETE', path: '/files/:filename',                         handler: 'deleteFile',             desc: 'Remove a file from the cache directory.' },
								{ method: 'POST',   path: '/files/move/:currentfilepathandname/:newfilepathandname', handler: 'moveFile', desc: "Move/rename a stored file and update its DB filepath (issue #333). Known broken: gin's :param can't contain \"/\", so any real subfolder path 404s, and MoveFile doesn't prefix fileSystemBaseDir, so even a bare filename hits the container's read-only root instead of the store/ volume." },
								{ method: 'GET',    path: '/admin',                                   handler: '—',                      desc: 'Serves the admin portal HTML page.' },
								{ method: 'GET',    path: '/settings',                                handler: '—',                      desc: 'Serves the settings HTML page.' },
								{ method: 'GET',    path: '/admin/dumpCache',                         handler: 'triggerCacheDump',       desc: 'Immediately clear all files from the cache directory.' },
								{ method: 'GET',    path: '/admin/reloadPlugins',                     handler: 'reloadPlugins',          desc: 'Reload plugins from disk unconditionally, without restarting the server.' },
								{ method: 'GET',    path: '/machines',                                handler: 'listMachines',           desc: 'List every machine registered in the known_machines allowlist.' },
								{ method: 'POST',   path: '/machines',                                handler: 'addMachine',             desc: 'Register a new machine. Body: {"ip": "...", "label": "..."}.' },
								{ method: 'DELETE', path: '/machines/:ip',                            handler: 'deleteMachine',          desc: 'Remove a machine from the allowlist by IP.' },
							] as row}
								<tr>
									<td class="py-2 pr-4">
										<span class="rounded px-1.5 py-0.5 font-mono text-xs font-bold
											{row.method === 'GET' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300' :
											 row.method === 'POST' ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300' :
											 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'}">
											{row.method}
										</span>
									</td>
									<td class="py-2 pr-4 font-mono text-xs text-gray-700 dark:text-gray-300">{row.path}</td>
									<td class="py-2 pr-4 font-mono text-xs text-gray-500 dark:text-gray-400">{row.handler}</td>
									<td class="py-2 text-xs text-gray-600 dark:text-gray-400">{row.desc}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				<h3 class="mb-2 mt-6 text-sm font-semibold text-gray-700 dark:text-gray-200">Upload — Form Fields</h3>
				<p class="mb-2 text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">POST /upload</code> expects
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">multipart/form-data</code> with the following fields:
				</p>
				<div class="overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="border-b border-gray-200 text-left dark:border-gray-700">
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Field</th>
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Type</th>
								<th class="pb-2 font-semibold text-gray-700 dark:text-gray-200">Notes</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-100 text-xs dark:divide-gray-700">
							{#each [
								{ field: 'file',          type: 'File',   notes: 'Required. The file to upload.' },
								{ field: 'claim_number',  type: 'string', notes: 'Claim identifier.' },
								{ field: 'claimant_name', type: 'string', notes: 'Full name of the claimant.' },
								{ field: 'date_of_injury',type: 'string', notes: 'Date of injury.' },
								{ field: 'employer',      type: 'string', notes: 'Employer name.' },
								{ field: 'adjuster',      type: 'string', notes: 'Adjuster name.' },
								{ field: 'support',       type: 'string', notes: 'Support contact.' },
								{ field: 'claim_type',    type: 'string', notes: 'Type of claim.' },
								{ field: 'jurisdiction',  type: 'string', notes: 'Jurisdiction.' },
								{ field: 'policy_number', type: 'string', notes: 'Insurance policy number.' },
								{ field: 'acts_id',       type: 'string', notes: 'ACTs system ID. Used as the foreign key linking files to metadata.' },
							] as row}
								<tr>
									<td class="py-1.5 pr-4 font-mono text-gray-700 dark:text-gray-300">{row.field}</td>
									<td class="py-1.5 pr-4 text-gray-500 dark:text-gray-400">{row.type}</td>
									<td class="py-1.5 text-gray-600 dark:text-gray-400">{row.notes}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				<h3 class="mb-2 mt-6 text-sm font-semibold text-gray-700 dark:text-gray-200">Allowed File Types</h3>
				<p class="text-sm text-gray-600 dark:text-gray-300">
					Files with any other extension are rejected with <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">400 Bad Request</code>.
					Files are also checked for PE and ELF executable magic bytes regardless of extension.
				</p>
				<div class="mt-2 flex flex-wrap gap-2">
					{#each ['.pdf', '.txt', '.doc', '.docx', '.xls', '.xlsx', '.csv', '.ppt', '.png', '.jpg', '.jpeg'] as ext}
						<span class="rounded bg-gray-100 px-2 py-0.5 font-mono text-xs dark:bg-gray-700 dark:text-gray-300">{ext}</span>
					{/each}
				</div>
			</section>

			<!-- ── CLI Tool ── -->
			<section id="cli" transition:fade={{ duration: 300 }} class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">CLI Tool</h2>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">cli/</code> is a small standalone Go
					module (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">dewey-cli</code>) that wraps
					every backend route above, so the API can be exercised during development without reaching
					for <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">curl</code> and hand-built
					multipart requests.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Build & run</h3>
				<pre class="mb-4 overflow-x-auto rounded-lg bg-gray-50 p-4 text-xs dark:bg-gray-900"><code class="text-gray-800 dark:text-gray-200">cd cli
go build -o dewey-cli .

# Defaults to http://localhost:8080 — override with DEWEY_HOST
DEWEY_HOST=http://&lt;host&gt;:8080 ./dewey-cli health</code></pre>
				<div class="mb-4 rounded-lg border border-yellow-200 bg-yellow-50 p-3 text-xs text-yellow-800 dark:border-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-300">
					<strong>Gotcha:</strong> a bare <code class="rounded bg-yellow-100 px-1 dark:bg-yellow-900">go build</code> in <code class="rounded bg-yellow-100 px-1 dark:bg-yellow-900">cli/</code> fails with
					<code class="rounded bg-yellow-100 px-1 dark:bg-yellow-900">pattern embedded_docs/admin_readme.md: no matching files found</code>
					unless <code class="rounded bg-yellow-100 px-1 dark:bg-yellow-900">package.sh</code> has run at least once first. The <code class="rounded bg-yellow-100 px-1 dark:bg-yellow-900">docs</code> command's
					<code class="rounded bg-yellow-100 px-1 dark:bg-yellow-900">go:embed</code> directive can't reach files outside its own module directory, so
					<code class="rounded bg-yellow-100 px-1 dark:bg-yellow-900">package.sh</code> stages copies of the root README and this admin README into
					<code class="rounded bg-yellow-100 px-1 dark:bg-yellow-900">cli/embedded_docs/</code> right before building. Run <code class="rounded bg-yellow-100 px-1 dark:bg-yellow-900">package.sh</code> first, or stage them by hand:
					<code class="rounded bg-yellow-100 px-1 dark:bg-yellow-900">mkdir -p cli/embedded_docs && cp README.md cli/embedded_docs/readme.md && cp frontend/doctooladmin/README.md cli/embedded_docs/admin_readme.md</code>.
				</div>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Commands</h3>
				<div class="mb-4 overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="border-b border-gray-200 text-left dark:border-gray-700">
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Command</th>
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Route</th>
								<th class="pb-2 font-semibold text-gray-700 dark:text-gray-200">Notes</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-100 text-xs dark:divide-gray-700">
							{#each [
								{ cmd: 'health',                          route: 'GET  /',                     notes: '' },
								{ cmd: 'version',                         route: 'GET  /version',               notes: '' },
								{ cmd: 'dump_cache',                      route: 'GET  /admin/dumpCache',       notes: '' },
								{ cmd: 'list_machines',                   route: 'GET  /machines',              notes: '' },
								{ cmd: 'add_machine <ip> <label>',        route: 'POST /machines',              notes: '' },
								{ cmd: 'delete_machine <ip>',             route: 'DELETE /machines/:ip',        notes: '' },
								{ cmd: 'list_files',                      route: 'GET  /files',                 notes: '' },
								{ cmd: 'get_file <filename>',             route: 'GET  /files/:filename/false', notes: '' },
								{ cmd: 'get_file_meta <filename>',        route: 'GET  /files/:filename/true',  notes: '' },
								{ cmd: 'delete_file <filename>',          route: 'DELETE /files/:filename',     notes: '' },
								{ cmd: 'upload <path> [field=value ...]', route: 'POST /upload',                notes: 'See metadata fields below.' },
								{ cmd: 'self_ip',                         route: '—',                           notes: 'Locally-determined outbound IP toward DEWEY_HOST — a starting guess for what to register in Known Machines, not a guarantee (NAT can rewrite the source address in transit).' },
								{ cmd: 'metrics',                         route: 'GET  /metrics',               notes: 'Live terminal metrics dashboard (issue #348). Polls every 5s; q to quit, arrows/jk/wheel to scroll.' },
								{ cmd: 'docs <readme|admin>',             route: '—',                           notes: 'Terminal markdown viewer (issue #379) for README.md or frontend/doctooladmin/README.md, baked into the binary at build time. q to quit, arrows/jk/wheel to scroll.' },
								{ cmd: 'soak_test [sim_days] [users] [seconds_per_sim_day]', route: '—',       notes: 'Runs soak_test.sh inside the backend container via podman exec. See Testing Tools below.' },
							] as row}
								<tr>
									<td class="py-2 pr-4 font-mono text-gray-700 dark:text-gray-300">{row.cmd}</td>
									<td class="py-2 pr-4 font-mono text-xs text-gray-500 dark:text-gray-400">{row.route}</td>
									<td class="py-2 text-gray-600 dark:text-gray-400">{row.notes}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Upload metadata</h3>
				<p class="mb-2 text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">upload</code> accepts any of the
					<a href="#api" class="underline">metadata fields</a> as trailing
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">field=value</code> arguments, in any
					order. Fields left out are simply not sent, same as leaving them blank in the admin portal's
					upload dialog. An unrecognised field name fails immediately rather than being silently
					dropped.
				</p>
				<pre class="overflow-x-auto rounded-lg bg-gray-50 p-4 text-xs dark:bg-gray-900"><code class="text-gray-800 dark:text-gray-200">./dewey-cli upload report.pdf \
  claim_number=CL-1024 \
  claimant_name="Jane Doe" \
  acts_id=A-88</code></pre>
			</section>

			<!-- ── MCP Server ── -->
			<section id="mcp" transition:fade={{ duration: 300 }} class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">MCP Server</h2>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">dewey-mcp/</code> is a standalone Go
					module implementing an MCP (<a href="https://modelcontextprotocol.io" class="underline" target="_blank" rel="noopener noreferrer">Model Context Protocol</a>)
					server, exposing Dewey and its Podman pod to AI model clients (Claude Desktop, Claude Code, etc.)
					as a set of callable tools. Unlike the REST API above, it communicates over stdio rather than
					HTTP — a client spawns the built binary as a subprocess per connection rather than dialing a port.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Build & run</h3>
				<pre class="mb-4 overflow-x-auto rounded-lg bg-gray-50 p-4 text-xs dark:bg-gray-900"><code class="text-gray-800 dark:text-gray-200">cd dewey-mcp
go build -o dewey-mcp .

# Talks to the backend at http://localhost:8080 by default — override with DEWEY_HOST
DEWEY_HOST=http://&lt;host&gt;:8080 ./dewey-mcp</code></pre>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					Point an MCP client's stdio transport at the built binary to connect — there's no port to
					publish or firewall rule to open.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Tools</h3>
				<div class="mb-4 overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="border-b border-gray-200 text-left dark:border-gray-700">
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Tool</th>
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Kind</th>
								<th class="pb-2 font-semibold text-gray-700 dark:text-gray-200">Description</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-100 text-xs dark:divide-gray-700">
							{#each [
								{ tool: 'is_up',                     kind: 'read-only',   desc: 'Checks whether the Dewey backend is reachable, via GET /.' },
								{ tool: 'version',                   kind: 'read-only',   desc: "Returns the backend's release version and the git branch it was built from." },
								{ tool: 'get_podman_health',         kind: 'read-only',   desc: 'Status of the dewey-pod Podman pod (podman pod ps).' },
								{ tool: 'get_podman_containers',     kind: 'read-only',   desc: 'Lists every running Podman container (podman ps).' },
								{ tool: 'get_podman_container_logs', kind: 'read-only',   desc: 'Recent log output of a single container, by name or ID.' },
								{ tool: 'restart_podman_container',  kind: 'destructive', desc: 'Restarts a single container by name or ID — interrupts whatever it was serving. The only non-idempotent tool here.' },
								{ tool: 'create_file',               kind: 'destructive', desc: 'Writes contents to a file at path, creating or truncating it.' },
								{ tool: 'read_file',                 kind: 'read-only',   desc: "Returns a file's contents." },
								{ tool: 'move_file',                 kind: 'destructive', desc: 'Moves/renames a file from src to dst. Skips rather than overwrites if dst already exists.' },
								{ tool: 'delete_file',               kind: 'destructive', desc: 'Removes a file.' },
							] as row}
								<tr>
									<td class="py-2 pr-4 font-mono text-gray-700 dark:text-gray-300">{row.tool}</td>
									<td class="py-2 pr-4">
										<span class="rounded px-1.5 py-0.5 font-mono text-xs font-bold
											{row.kind === 'read-only' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300' :
											 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'}">
											{row.kind}
										</span>
									</td>
									<td class="py-2 text-gray-600 dark:text-gray-400">{row.desc}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Path-traversal protection</h3>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">read_file</code>,
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">move_file</code>, and
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">delete_file</code> resolve every path
					argument through <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">resolveSafePath</code>,
					which rejects anything that resolves outside the server's working directory — an absolute path
					or a <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">../</code> escape — mirroring the
					backend's own <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">resolveStorePath</code>
					guard used for uploaded file paths. Without it, any connected MCP client would have
					unrestricted read/move/delete access to whatever the server process itself can reach on the
					host.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Packaging</h3>
				<p class="text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">package.sh</code> (repo root)
					cross-compiles <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">dewey-mcp</code> for
					linux/amd64 and bundles the binary alongside
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">dewey-cli</code> in the shipped
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">.tar.gz</code>.
				</p>
			</section>

			<!-- ── Testing Tools ── -->
			<section id="testing" transition:fade={{ duration: 300 }} class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Testing Tools</h2>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">backend/testing_tooling/</code> holds the
					BIT suite plus two load-generation scripts, all run from inside the backend container so their
					requests come from an already-<a href="#access" class="underline">allowlisted</a> source:
				</p>
				<ul class="mb-6 space-y-1 text-sm text-gray-600 dark:text-gray-300">
					<li><strong class="text-gray-700 dark:text-gray-200">test_suite.sh</strong> — the BIT (built-in test) suite: a black-box pass over the whole HTTP API, run automatically at every server startup. What the next subsection documents.</li>
					<li><strong class="text-gray-700 dark:text-gray-200">load_test.sh</strong> — a fixed-size burst of uploads (plus a retrieval pass) from a single source, done in seconds. Good for a quick sanity check.</li>
					<li><strong class="text-gray-700 dark:text-gray-200">soak_test.sh</strong> — sustained, multi-user, day/night-shaped traffic over a configurable duration (issue #311).</li>
				</ul>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">BIT suite (test_suite.sh)</h3>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					Runs automatically once the server finishes starting up — backgrounded (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">main.go</code>'s
					BITs goroutine) so it doesn't block the server from listening. A failure only logs a
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">Warn</code>, it doesn't crash the server —
					this is a self-test, not a startup gate. Run it manually the same way, against any target:
				</p>
				<pre class="mb-4 overflow-x-auto rounded-lg bg-gray-900 p-3 text-xs text-gray-100"><code>bash backend/testing_tooling/test_suite.sh [base_url]  # defaults to http://localhost:8080

# or, against a running deployment, from inside the backend container:
podman exec cross-doc-tool-dev bash testing_tooling/test_suite.sh</code></pre>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					Structured <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">PASS</code>/<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">FAIL</code>/<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">SKIP</code>
					lines go to stdout, human-readable progress to stderr — both are also teed into a timestamped log
					under <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/app/logs/bits/</code>, a persisted volume
					that survives a container restart (issue #213), so a failed self-test on a redeployed container is
					still debuggable afterward.
				</p>
				<p class="mb-2 text-sm text-gray-600 dark:text-gray-300">What it checks, in run order:</p>
				<ol class="mb-6 space-y-2 text-sm text-gray-600 dark:text-gray-300">
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">1</span><span><strong class="text-gray-700 dark:text-gray-200">Endpoint health</strong> — <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/</code>, <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/files</code>, <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/admin/dumpCache</code> all respond.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">2</span><span><strong class="text-gray-700 dark:text-gray-200">JSON upload</strong> — the JSON-only branch of <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/upload</code> accepts valid JSON and rejects malformed JSON.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">3</span><span><strong class="text-gray-700 dark:text-gray-200">Multipart upload</strong> — every fixture file type uploads successfully with randomized metadata.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">4</span><span><strong class="text-gray-700 dark:text-gray-200">Upload error cases</strong> — a bad content-type and a missing file field are both rejected.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">5</span><span><strong class="text-gray-700 dark:text-gray-200">Disallowed extension</strong> — <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">.sh</code>/<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">.exe</code> uploads are rejected.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">6</span><span><strong class="text-gray-700 dark:text-gray-200">ELF rejection</strong> — an ELF binary disguised with a <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">.txt</code> extension is still detected and blocked.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">7</span><span><strong class="text-gray-700 dark:text-gray-200">PDF JavaScript rejection</strong> — a PDF with an embedded <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/OpenAction</code> trigger is rejected (issue #245).</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">8</span><span><strong class="text-gray-700 dark:text-gray-200">Path traversal</strong> — <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">../</code>-style paths in <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">GET</code>/<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">DELETE</code> requests are blocked.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">9</span><span><strong class="text-gray-700 dark:text-gray-200">SHA256 upload verification</strong> — the server-reported hash in the upload response matches the file's real hash.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">10</span><span><strong class="text-gray-700 dark:text-gray-200">Duplicate upload</strong> — uploading the same file twice produces two distinct stored filenames.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">11</span><span><strong class="text-gray-700 dark:text-gray-200">Roundtrip integrity</strong> — a downloaded file's hash matches what was uploaded, via <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">GET /files/:filename/false</code>.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">12</span><span><strong class="text-gray-700 dark:text-gray-200">Store path verification</strong> — checks the store <em>on disk</em> for the file, not just through the API (issue #270). The roundtrip check above alone can't catch a file that's recorded in the DB but never actually copied to <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">fileSystemBaseDir</code> — <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">locateFile</code> checks the cache first, so a passing roundtrip can be served entirely from there. This polls briefly (the store copy lands asynchronously, behind <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">postProcessingSem</code>), then confirms the store copy's content matches via SHA256.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">13</span><span><strong class="text-gray-700 dark:text-gray-200">Metadata retrieval</strong> — <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">GET /files/:filename/true</code> returns the expected fields, an unknown <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">:meta</code> value gets 400, and a non-existent file gets 404.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">14</span><span><strong class="text-gray-700 dark:text-gray-200">Catalog</strong> — <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">GET /files</code> lists what was just uploaded.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">15</span><span><strong class="text-gray-700 dark:text-gray-200">Delete</strong> — deleting a real file succeeds, and deleting a nonexistent one returns 404.</span></li>
				</ol>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Why soak_test.sh exists</h3>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					<a href="#daemon" class="underline">The daemon</a>'s tick interval reacts to an EMA-smoothed
					upload/retrieval rate and the active-user count — behavior that only meaningfully diverges from
					steady state under sustained, <em>varying</em> load. A short burst can't exercise that, or the
					slower trends (cache-clean cadence, memory growth) that only show up over a long run.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Simulating multiple users</h3>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					Every container in <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">dewey-pod</code> shares
					one network namespace, so extra containers wouldn't produce distinct source IPs for
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">ActiveUserCount()</code> to see. Instead,
					each simulated user binds its requests to its own loopback alias
					(<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">127.0.0.2</code>,
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">127.0.0.3</code>, …) via
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">curl --interface</code>, and
					registers/deregisters that IP through the <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/machines</code>
					routes at start and exit. The backend genuinely sees a different
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">RemoteAddr</code> per user with no network
					changes required.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Traffic shape</h3>
				<p class="mb-2 text-sm text-gray-600 dark:text-gray-300">
					Each user's request cadence follows a diurnal curve — quiet overnight, busy through the
					afternoon — rather than firing at a constant rate:
				</p>
				<pre class="mb-4 overflow-x-auto rounded-lg bg-gray-50 p-4 text-xs dark:bg-gray-900"><code class="text-gray-800 dark:text-gray-200">multiplier(hour) = floor + (1 - floor) &middot; max(0, sin(&pi;&middot;(hour-9)/12))
interval(hour)   = SECONDS_PER_SIM_DAY / (peak_requests_per_day &middot; multiplier(hour))</code></pre>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					The bump is positive only for simulated hour 9 through 21, peaking at 15:00; outside that
					window the rate sits at a low floor rather than stopping outright. The interval is derived
					as a requests-per-simulated-day target rather than a fixed number of seconds, so the ramp
					stays meaningful at any compression factor — jittered &plusmn;50%, and capped at whatever
					real time is left in the run so a quiet-hour interval can never sleep past the run's own end.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Time compression</h3>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">SECONDS_PER_SIM_DAY</code> controls how
					many real seconds map to one simulated day — the same parameter either way, not a separate
					"mode". The default, <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">60</code>, compresses
					30 simulated days into about 30 real minutes. Passing
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">86400</code> gives true real-time pacing —
					useful for an actual multi-day soak run, at the cost of it taking that many real days to finish.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Metrics log</h3>
				<p class="mb-2 text-sm text-gray-600 dark:text-gray-300">
					A separate background loop samples <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/metrics</code>
					on a fixed real-time cadence (every 10s, independent of compression) and appends one row to
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/app/logs/soak_metrics_&lt;run&gt;.csv</code>
					— plain CSV, readable directly by a plotting script:
				</p>
				<pre class="mb-4 overflow-x-auto rounded-lg bg-gray-50 p-4 text-xs dark:bg-gray-900"><code class="text-gray-800 dark:text-gray-200">timestamp,sim_day,sim_hour,time_til_next_tick,cache_clean_cycles_total,
ram_usage_mb,heap_usage_mb,goroutines,open_fds,connected_users,upload_rate</code></pre>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">cache_clean_cycles_total</code> reads the
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">app_cache_clean_cycles_total</code> gauge
					— a counter incremented once per <a href="#daemon" class="underline">daemon tick</a> that runs a
					cache clear, added alongside the other Prometheus metrics so cache-clean cadence is visible
					over a long run rather than only the current cache size at any one instant.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Running it</h3>
				<pre class="overflow-x-auto rounded-lg bg-gray-50 p-4 text-xs dark:bg-gray-900"><code class="text-gray-800 dark:text-gray-200"># via the CLI, from the deploy host
dewey-cli soak_test 30 5 60      # default: 30 sim days, 5 users, 60s/simday
dewey-cli soak_test 7 5 86400    # the real thing: 7 real days, true pacing

# or directly, from inside the backend container
podman exec cross-doc-tool-dev ./testing_tooling/soak_test.sh</code></pre>
			</section>

			<!-- ── Access Control ── -->
			<section id="access" transition:fade={{ duration: 300 }} class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Access Control</h2>
				<p class="mb-3 text-sm text-gray-600 dark:text-gray-300">
					Dewey runs on a closed network of machines talking to each other — no route is reachable
					from outside that network. Instead of per-user authentication, access is gated by an
					IP allowlist: the <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">known_machines</code>
					table, checked by the <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">logConnections</code>
					middleware (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">main.go</code>) ahead of every
					other handler.
				</p>
				<p class="mb-3 text-sm text-gray-600 dark:text-gray-300">
					On each request the middleware resolves the caller's address via <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">resolveClientIP</code>,
					which parses <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">c.Request.RemoteAddr</code> directly using
					Go's <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">net/netip</code> package rather than gin's own
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">c.ClientIP()</code> — the latter calls
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">net.ParseIP</code>, which silently returns an empty
					string for any zone-qualified address (e.g. <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">fe80::...%eth0</code>),
					which is exactly what rootless Podman's <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">pasta</code> network
					helper presents for host-to-forwarded-port connections (issue #315).
				</p>
				<p class="mb-3 text-sm text-gray-600 dark:text-gray-300">
					Link-local IPv6 addresses (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">fe80::/10</code>) are treated
					as host-equivalent and skip the allowlist check entirely — only reachable from the local link, and tied to
					the host's specific network interface rather than something that could be usefully pre-registered the way
					loopback is. Anything else goes through <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">checkKnownMachine</code>:
					an unregistered IP gets <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">403 Forbidden</code> before
					reaching the route handler, while a registered IP proceeds and <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">logMachineIP</code>
					bumps <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">last_seen_at</code> for it — this doubles as the
					connection log (who talked to the server, and when).
				</p>
				<p class="mb-3 text-sm text-gray-600 dark:text-gray-300">
					The server calls <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">r.SetTrustedProxies(nil)</code> so
					gin ignores <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">X-Forwarded-For</code>/<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">X-Real-IP</code>
					headers — without this, any machine could set that header and spoof its way past the allowlist,
					since there's no reverse proxy in front of this pod to strip it.
				</p>
				<div class="rounded-lg border border-yellow-200 bg-yellow-50 p-3 text-xs text-yellow-800 dark:border-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-300">
					<strong>Scope:</strong> this is IP-based, not identity-based — there's no login or credential.
					It stops outside machines from reaching the API cold, but anything already on the network
					that can claim a registered IP (DHCP collision, static IP reuse) gets full access with no
					further check. That trade-off is intentional given the closed-network threat model; it is
					not a substitute for real authentication if Dewey is ever exposed more broadly.
				</div>
				<h3 class="mb-2 mt-4 text-sm font-semibold text-gray-700 dark:text-gray-200">Managing the allowlist</h3>
				<p class="text-sm text-gray-600 dark:text-gray-300">
					Machines can be listed, added, and removed via the <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/machines</code>
					routes above, or through the Known Machines page in the admin portal. Because those routes are
					gated by the same allowlist, the very first machine can't bootstrap itself through the API —
					register it directly against <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">known_machines</code>
					(a manual <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">INSERT</code>, or a CLI flag if one is added)
					before anything else can reach the server.
				</p>
			</section>

			<!-- ── Plugin System ── -->
			<section id="plugins" transition:fade={{ duration: 300 }} class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Plugin System</h2>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					Plugins are Lua scripts dropped into <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">./plugins/</code>.
					They are loaded at startup by the <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">PluginManger</code> against
					a dynamic registry of hook names Go registers up front — there's no fixed set of plugin "types" baked into the
					loader. Every plugin exports <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">WhoAmI</code>, plus one function
					per hook it wants to attach to, named exactly after that hook. A single file can implement more than one hook.
				</p>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					Plugins can be added, removed, or edited after startup without restarting the server (issue #325) — see the
					<a href="#daemon" class="underline">Daemon</a> section for how change-detection and reload work.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Hooks</h3>
				<div class="mb-4 overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="border-b border-gray-200 text-left dark:border-gray-700">
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Hook</th>
								<th class="pb-2 font-semibold text-gray-700 dark:text-gray-200">When it runs</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-100 text-xs dark:divide-gray-700">
							<tr><td class="py-2 pr-4 font-mono text-gray-700 dark:text-gray-300">OnUpload</td><td class="py-2 text-gray-600 dark:text-gray-400">Once per upload, during async post-processing, before OnFilter. Runs after the 200 OK is already sent, so it can't veto the upload — tag-only (e.g. write into <code class="rounded bg-gray-100 px-0.5 dark:bg-gray-700">entry.Meta</code>).</td></tr>
							<tr><td class="py-2 pr-4 font-mono text-gray-700 dark:text-gray-300">OnFilter</td><td class="py-2 text-gray-600 dark:text-gray-400">Once per upload, during async post-processing. Can mutate the destination path (<code class="rounded bg-gray-100 px-0.5 dark:bg-gray-700">entry.Path</code>).</td></tr>
							<tr><td class="py-2 pr-4 font-mono text-gray-700 dark:text-gray-300">OnDelete</td><td class="py-2 text-gray-600 dark:text-gray-400">Once per delete request, synchronously, before anything is removed. Calling <code class="rounded bg-gray-100 px-0.5 dark:bg-gray-700">error(...)</code> vetoes the deletion — the caller gets a 403 instead.</td></tr>
							<tr><td class="py-2 pr-4 font-mono text-gray-700 dark:text-gray-300">OnInit</td><td class="py-2 text-gray-600 dark:text-gray-400">Once at server startup. Registered and invoked; no shipped example plugin.</td></tr>
							<tr><td class="py-2 pr-4 font-mono text-gray-700 dark:text-gray-300">OnTick</td><td class="py-2 text-gray-600 dark:text-gray-400">Once per daemon tick. Registered and invoked; no shipped example plugin.</td></tr>
						</tbody>
					</table>
				</div>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Plugin Structure</h3>
				<p class="mb-2 text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">WhoAmI</code> reports salience only — which hook(s) a plugin
					attaches to comes entirely from the hook-named functions it defines, e.g. <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">OnFilter(entry)</code>:
				</p>
				<pre class="overflow-x-auto rounded-lg bg-gray-50 p-4 text-xs dark:bg-gray-900"><code class="text-gray-800 dark:text-gray-200">-- Salience determines this plugin's run order relative to other OnFilter
-- plugins (higher runs first). WhoAmI reports salience only — which hook
-- this plugin attaches to comes from the OnFilter function name below.
Salience = 10

function WhoAmI()
    return Salience
end

-- OnFilter receives the file entry table and returns a (possibly modified)
-- copy.
function OnFilter(entry)
    -- entry.Filename  — timestamped filename (e.g. "1700000000_report.pdf")
    -- entry.Act       — ACTs ID from upload metadata
    -- entry.Hash      — SHA-256 hash of the file
    -- entry.Path      — destination path (OnFilter should modify this)
    -- entry.Meta      — metadata string
    -- entry.Barcode   — decoded barcode text, or nil if none found

    -- Example: sort PDFs into a subdirectory
    if string.match(entry.Filename, "%.pdf$") then
        entry.Path = "pdf/" .. entry.Filename
    end

    return entry
end</code></pre>

				<h3 class="mb-2 mt-4 text-sm font-semibold text-gray-700 dark:text-gray-200">Salience</h3>
				<p class="text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">WhoAmI</code>'s return value is the salience.
					Higher numbers run first. Use salience to control ordering when multiple plugins attach to the same hook.
					Plugins with equal salience run in an undefined order. Each plugin runs in its own isolated Lua state
					(via a per-plugin <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">sync.Pool</code>), so one plugin's globals can't leak into another's.
				</p>
			</section>

			<!-- ── Database ── -->
			<section id="database" transition:fade={{ duration: 300 }} class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Database</h2>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					MySQL. Connection string is read from the <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">DB_DSN</code> environment variable.
					Falls back to <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">root:dewey@tcp(127.0.0.1:3306)/deweyRecords</code> for local development.
					Schema is applied automatically on startup via <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">Migrate()</code>.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">files</h3>
				<div class="mb-4 overflow-x-auto">
					<table class="w-full text-xs">
						<thead>
							<tr class="border-b border-gray-200 text-left dark:border-gray-700">
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Column</th>
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Type</th>
								<th class="pb-2 font-semibold text-gray-700 dark:text-gray-200">Notes</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-100 dark:divide-gray-700">
							{#each [
								{ col: 'id',          type: 'INT AUTO_INCREMENT',  notes: 'Primary key.' },
								{ col: 'filename',    type: 'VARCHAR(255)',         notes: 'Timestamped filename as stored on disk.' },
								{ col: 'acts_id',     type: 'VARCHAR(100)',         notes: 'Foreign reference to the meta table. Indexed.' },
								{ col: 'sha256_hash', type: 'CHAR(64)',             notes: 'SHA-256 of the file content at upload time.' },
								{ col: 'created_at',  type: 'VARCHAR(35)',          notes: 'RFC3339 timestamp of ingest.' },
								{ col: 'filepath',    type: 'TEXT',                 notes: 'Path within ./store as set by the plugin chain.' },
								{ col: 'is_deleted',  type: 'TINYINT(1)',           notes: 'Soft delete flag. 0 = active, 1 = deleted.' },
								{ col: 'barcode',     type: 'VARCHAR(100)',         notes: 'Decoded barcode text, or NULL if none found.' },
							] as row}
								<tr>
									<td class="py-1.5 pr-4 font-mono text-gray-700 dark:text-gray-300">{row.col}</td>
									<td class="py-1.5 pr-4 text-gray-500 dark:text-gray-400">{row.type}</td>
									<td class="py-1.5 text-gray-600 dark:text-gray-400">{row.notes}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">meta</h3>
				<div class="overflow-x-auto">
					<table class="w-full text-xs">
						<thead>
							<tr class="border-b border-gray-200 text-left dark:border-gray-700">
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Column</th>
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Type</th>
								<th class="pb-2 font-semibold text-gray-700 dark:text-gray-200">Notes</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-100 dark:divide-gray-700">
							{#each [
								{ col: 'id',            type: 'INT AUTO_INCREMENT', notes: 'Primary key.' },
								{ col: 'claim_number',  type: 'VARCHAR(100)',        notes: 'Indexed for fast lookup.' },
								{ col: 'claimant_name', type: 'VARCHAR(255)',        notes: '' },
								{ col: 'date_of_injury',type: 'DATE',               notes: '' },
								{ col: 'employer',      type: 'VARCHAR(255)',        notes: '' },
								{ col: 'adjuster',      type: 'VARCHAR(255)',        notes: '' },
								{ col: 'support',       type: 'VARCHAR(255)',        notes: '' },
								{ col: 'claim_type',    type: 'VARCHAR(100)',        notes: '' },
								{ col: 'jurisdiction',  type: 'VARCHAR(100)',        notes: '' },
								{ col: 'policy_number', type: 'VARCHAR(100)',        notes: '' },
								{ col: 'acts_id',       type: 'VARCHAR(100)',        notes: 'Links to files.acts_id. Indexed.' },
							] as row}
								<tr>
									<td class="py-1.5 pr-4 font-mono text-gray-700 dark:text-gray-300">{row.col}</td>
									<td class="py-1.5 pr-4 text-gray-500 dark:text-gray-400">{row.type}</td>
									<td class="py-1.5 text-gray-600 dark:text-gray-400">{row.notes}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				<p class="mt-4 mb-6 text-sm text-gray-600 dark:text-gray-300">
					Deletes are soft — the <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">is_deleted</code> flag is set to
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">1</code> rather than the row being removed.
					All queries filter on <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">is_deleted = 0</code>.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">known_machines</h3>
				<p class="mb-2 text-sm text-gray-600 dark:text-gray-300">
					Backs the <a href="#access" class="underline">Access Control</a> allowlist — see that section for how it's used.
				</p>
				<div class="overflow-x-auto">
					<table class="w-full text-xs">
						<thead>
							<tr class="border-b border-gray-200 text-left dark:border-gray-700">
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Column</th>
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Type</th>
								<th class="pb-2 font-semibold text-gray-700 dark:text-gray-200">Notes</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-100 dark:divide-gray-700">
							{#each [
								{ col: 'id',           type: 'INT AUTO_INCREMENT', notes: 'Primary key.' },
								{ col: 'ip',           type: 'VARCHAR(45)',        notes: 'Unique. Source IP checked on every request. Sized for IPv4 and IPv6.' },
								{ col: 'label',        type: 'VARCHAR(255)',       notes: 'Human-readable name for the machine (e.g. "vm-2").' },
								{ col: 'added_at',     type: 'DATETIME',           notes: 'Defaults to the time the machine was registered.' },
								{ col: 'last_seen_at', type: 'DATETIME',           notes: 'Updated on every authorized request from this IP. NULL until first seen.' },
							] as row}
								<tr>
									<td class="py-1.5 pr-4 font-mono text-gray-700 dark:text-gray-300">{row.col}</td>
									<td class="py-1.5 pr-4 text-gray-500 dark:text-gray-400">{row.type}</td>
									<td class="py-1.5 text-gray-600 dark:text-gray-400">{row.notes}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</section>

			<!-- ── Daemon ── -->
			<section id="daemon" transition:fade={{ duration: 300 }} class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Daemon</h2>
				<p class="mb-3 text-sm text-gray-600 dark:text-gray-300">
					A background goroutine (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">startDaemon</code>) fires every
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">daemonTickTime</code> hours. On each tick it:
				</p>
				<ol class="mb-4 space-y-1 text-sm text-gray-600 dark:text-gray-300">
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">1</span><span>Clears all files from <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">./cache</code> (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">dumpCache</code>).</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">2</span><span>Creates a timestamped zip of <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">./store</code> in <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">./backup</code> (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">save</code>).</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">3</span><span>Checks whether the plugin directory changed since the last tick (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">HavePluginsChanged</code>, SHA-256 hash per file) and reloads plugins (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">ReloadPlugins</code>) only if something was added, removed, or edited — issue #325.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">4</span><span>Runs all registered OnTick plugins.</span></li>
				</ol>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">ReloadPlugins</code> resets loaded plugins and hook
					attachments and re-runs <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">LoadPlugins</code>, but leaves
					the registered hook <em>names</em> (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">OnUpload</code>,
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">OnFilter</code>, etc.) untouched — those are declared
					once at startup (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">main.go</code>), not per-load state.
					The same reload can also be forced on demand via
					<a href="#api" class="underline"><code class="rounded bg-gray-100 px-1 dark:bg-gray-700">GET /admin/reloadPlugins</code></a>,
					which reloads unconditionally rather than checking for changes first.
				</p>
				<p class="text-sm text-gray-600 dark:text-gray-300">
					The daemon uses an <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">observableTicker</code> wrapper around
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">time.Ticker</code>, which exposes
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">Remaining()</code> — the time until the next tick.
					This is surfaced via the <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">TimeUntilNextTick()</code> helper
					for use in API responses or metrics.
					The daemon stops cleanly when the server context is cancelled.
				</p>
			</section>

			<!-- ── Logging ── -->
			<section id="logging" transition:fade={{ duration: 300 }} class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Logging</h2>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					A custom logger in <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">log.go</code> writes to both the terminal and a
					date-rotating log file in <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">./logs/</code>.
					Terminal output is colourised; file output is plain text.
				</p>
				<div class="mb-4 overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="border-b border-gray-200 text-left dark:border-gray-700">
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Function</th>
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Level</th>
								<th class="pb-2 font-semibold text-gray-700 dark:text-gray-200">Use for</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-100 text-xs dark:divide-gray-700">
							<tr><td class="py-2 pr-4 font-mono text-gray-700 dark:text-gray-300">Debug(msg)</td><td class="py-2 pr-4 text-gray-500">DEBUG</td><td class="py-2 text-gray-600 dark:text-gray-400">Verbose internal state, low noise in production.</td></tr>
							<tr><td class="py-2 pr-4 font-mono text-gray-700 dark:text-gray-300">Info(msg)</td><td class="py-2 pr-4 text-gray-500">INFO</td><td class="py-2 text-gray-600 dark:text-gray-400">Key lifecycle events.</td></tr>
							<tr><td class="py-2 pr-4 font-mono text-gray-700 dark:text-gray-300">Warn(msg)</td><td class="py-2 pr-4 text-gray-500">WARN</td><td class="py-2 text-gray-600 dark:text-gray-400">Recoverable errors — plugin failures, file not found, etc.</td></tr>
							<tr><td class="py-2 pr-4 font-mono text-gray-700 dark:text-gray-300">Fatal(msg)</td><td class="py-2 pr-4 text-gray-500">FATAL</td><td class="py-2 text-gray-600 dark:text-gray-400">Unrecoverable errors. Closes the log and calls os.Exit(1).</td></tr>
							<tr><td class="py-2 pr-4 font-mono text-gray-700 dark:text-gray-300">Ok(msg)</td><td class="py-2 pr-4 text-gray-500">—</td><td class="py-2 text-gray-600 dark:text-gray-400">Green checkmark for startup success events. Terminal only.</td></tr>
							<tr><td class="py-2 pr-4 font-mono text-gray-700 dark:text-gray-300">Section(title)</td><td class="py-2 pr-4 text-gray-500">—</td><td class="py-2 text-gray-600 dark:text-gray-400">Styled divider for separating startup phases. Terminal only.</td></tr>
							<tr><td class="py-2 pr-4 font-mono text-gray-700 dark:text-gray-300">Banner()</td><td class="py-2 pr-4 text-gray-500">—</td><td class="py-2 text-gray-600 dark:text-gray-400">Startup banner. Called once after InitLogger.</td></tr>
						</tbody>
					</table>
				</div>
				<p class="text-sm text-gray-600 dark:text-gray-300">
					Log files rotate daily. The filename format is
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">logs/app-YYYY-MM-DD.log</code>.
					Initialise the logger with <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">InitLogger("logs", "app")</code>
					and defer <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">logger.Close()</code>.
				</p>
			</section>

			<!-- ── Configuration ── -->
			<section id="config" transition:fade={{ duration: 300 }} class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Configuration</h2>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					Defaults live in <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">backend/config.json</code>,
					read once into these package-level vars at startup by <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">load()</code>
					(<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">consts.go</code>). There's no runtime override
					endpoint — changing a value means editing <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">config.json</code> and redeploying.
				</p>
				<div class="overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="border-b border-gray-200 text-left dark:border-gray-700">
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Constant</th>
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Default</th>
								<th class="pb-2 font-semibold text-gray-700 dark:text-gray-200">Description</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-100 text-xs dark:divide-gray-700">
							{#each [
								{ name: 'uploadDir',                   default: './cache', desc: 'Temporary landing directory for uploaded files.' },
								{ name: 'fileSystemBaseDir',           default: './store', desc: 'Permanent storage root. Plugins set paths relative to this.' },
								{ name: 'backupDir',                   default: './backup', desc: 'Destination for zip backups created on each daemon tick.' },
								{ name: 'daemonTickTime',              default: '1 (minute)', desc: 'Base interval between daemon ticks (cache clear + backup); computeTickInterval scales this up under active load — see Daemon section.' },
								{ name: 'alpha',                       default: '1', desc: 'Tick-scaling: extra seconds added per active user.' },
								{ name: 'beta',                        default: '1', desc: 'Tick-scaling: extra seconds added per unit of smoothed upload/retrieval rate.' },
								{ name: 'tBase',                       default: '5', desc: 'Tick-scaling: baseline interval in seconds when the system is idle.' },
								{ name: 'tickMax',                     default: '500', desc: 'Tick-scaling: upper bound (seconds) the computed interval is clamped to.' },
								{ name: 'tickMin',                     default: '1', desc: 'Tick-scaling: lower bound (seconds) the computed interval is clamped to.' },
								{ name: 'maxFileSize',                 default: '50 MB', desc: 'Maximum upload size enforced by the HTTP server.' },
								{ name: 'portNumber',                  default: '8080', desc: 'Port the backend listens on.' },
								{ name: 'appVersion',                  default: '0.3.0', desc: 'App release version shown in the dashboard topbar; bumped by hand per release.' },
								{ name: 'rateLimitPerSecond',          default: '80', desc: 'Global (not per-IP) token-bucket refill rate, in requests/second — shared across every client hitting the server.' },
								{ name: 'rateLimitBurst',              default: '120', desc: 'Burst allowance on top of the refill rate, also shared globally.' },
								{ name: 'maxOpenDBConnections',        default: '10', desc: 'Max simultaneous open DB connections.' },
								{ name: 'maxIdleDBConnections',        default: '10', desc: 'Max idle connections kept in the pool.' },
								{ name: 'dbConnectionTimeoutMultiplier', default: '2 (min)', desc: 'Connection lifetime before it is recycled.' },
								{ name: 'pluginDir',                   default: './plugins', desc: 'Directory scanned for Lua plugin files at startup.' },
								{ name: 'pluginScratchDir',            default: './plugin-scratch', desc: 'Sandbox directory plugins\' files.read/files.write are confined to — kept separate from store/cache/backup so a plugin can never reach a user\'s actual documents.' },
								{ name: 'maxConcurrentPostProcessing', default: '4', desc: 'Max concurrent post-processing goroutines (barcode scan + Lua plugins + disk copy) an upload burst can run at once.' },
							] as row}
								<tr>
									<td class="py-1.5 pr-4 font-mono text-gray-700 dark:text-gray-300">{row.name}</td>
									<td class="py-1.5 pr-4 text-gray-500 dark:text-gray-400">{row.default}</td>
									<td class="py-1.5 text-gray-600 dark:text-gray-400">{row.desc}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				<h3 class="mb-2 mt-6 text-sm font-semibold text-gray-700 dark:text-gray-200">Environment Variables</h3>
				<div class="overflow-x-auto">
					<table class="w-full text-xs">
						<thead>
							<tr class="border-b border-gray-200 text-left dark:border-gray-700">
								<th class="pb-2 pr-4 font-semibold text-gray-700 dark:text-gray-200">Variable</th>
								<th class="pb-2 font-semibold text-gray-700 dark:text-gray-200">Description</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-gray-100 dark:divide-gray-700">
							<tr>
								<td class="py-1.5 pr-4 font-mono text-gray-700 dark:text-gray-300">DB_DSN</td>
								<td class="py-1.5 text-gray-600 dark:text-gray-400">MySQL data source name. If unset, defaults to the local dev DSN.</td>
							</tr>
							<tr>
								<td class="py-1.5 pr-4 font-mono text-gray-700 dark:text-gray-300">DEWEY_POD_CPUS</td>
								<td class="py-1.5 text-gray-600 dark:text-gray-400">Pod CPU resource limit override for <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">deploy.sh</code> (default: <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">4</code>).</td>
							</tr>
							<tr>
								<td class="py-1.5 pr-4 font-mono text-gray-700 dark:text-gray-300">DEWEY_POD_MEMORY</td>
								<td class="py-1.5 text-gray-600 dark:text-gray-400">Pod memory resource limit override for <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">deploy.sh</code> (default: <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">4g</code>).</td>
							</tr>
						</tbody>
					</table>
				</div>
			</section>

			<!-- ── Deployment ── -->
			<section id="deployment" transition:fade={{ duration: 300 }} class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Deployment</h2>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					All services run in a single Podman Pod. The <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">deploy.sh</code>
					script at the repo root handles the full build and launch sequence.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Services in the Pod</h3>
				<ul class="mb-4 space-y-1 text-sm text-gray-600 dark:text-gray-300">
					<li><strong class="text-gray-700 dark:text-gray-200">Backend</strong> — Go binary, port 8080</li>
					<li><strong class="text-gray-700 dark:text-gray-200">Frontend</strong> — Node/SvelteKit, port 3000</li>
					<li><strong class="text-gray-700 dark:text-gray-200">Prometheus</strong> — metrics scraper, port 9090</li>
					<li><strong class="text-gray-700 dark:text-gray-200">Database</strong> — MySQL (managed externally or as a container)</li>
				</ul>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Quick deploy</h3>
				<pre class="overflow-x-auto rounded-lg bg-gray-50 p-4 text-xs dark:bg-gray-900"><code class="text-gray-800 dark:text-gray-200">bash deploy.sh
# Wipe the mysql-data volume before starting (opt-in — normally data persists):
bash deploy.sh --reset-db</code></pre>

				<h3 class="mb-2 mt-4 text-sm font-semibold text-gray-700 dark:text-gray-200">Persistence</h3>
				<p class="mb-2 text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">deploy.sh</code> mounts named Podman
					volumes so data survives a pod recreation instead of living only in a container's
					writable layer:
				</p>
				<ul class="mb-2 space-y-1 text-sm text-gray-600 dark:text-gray-300">
					<li><code class="rounded bg-gray-100 px-1 dark:bg-gray-700">mysql-data</code> → <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/var/lib/mysql</code></li>
					<li><code class="rounded bg-gray-100 px-1 dark:bg-gray-700">dewey-store</code> → <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/app/store</code></li>
					<li><code class="rounded bg-gray-100 px-1 dark:bg-gray-700">dewey-cache</code> → <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/app/cache</code></li>
					<li><code class="rounded bg-gray-100 px-1 dark:bg-gray-700">dewey-backup</code> → <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/app/backup</code></li>
					<li><code class="rounded bg-gray-100 px-1 dark:bg-gray-700">dewey-logs</code> → <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/app/logs</code></li>
					<li><code class="rounded bg-gray-100 px-1 dark:bg-gray-700">prometheus-data</code> → <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">/prometheus</code> (Prometheus's own metrics history; always persists, untouched by any of the flags below)</li>
				</ul>
				<p class="mb-2 text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">--reset-db</code> and the store/cache
					wipe (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">--keep-data</code> /
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">--wipe-data</code>) are independent —
					running one without the other can leave the database pointing at files that no longer
					exist on disk, or files on disk with no database record.
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">--clean-slate</code> forces both
					wipes together instead, gated behind a typed <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">yes</code>
					confirmation at an interactive terminal (it refuses to run non-interactively, since this
					permanently destroys every stored file and its metadata):
				</p>
				<pre class="overflow-x-auto rounded-lg bg-gray-50 p-4 text-xs dark:bg-gray-900"><code class="text-gray-800 dark:text-gray-200">bash deploy.sh --clean-slate</code></pre>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">package.sh</code>'s generated
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">run.sh</code> uses
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">podman play kube --replace</code>, so
					redeploying a new bundle on the server doesn't require tearing the pod down first — and
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">--replace</code> leaves the named
					volumes untouched, so data carries over between deploys there too.
				</p>

				<h3 class="mb-2 mt-4 text-sm font-semibold text-gray-700 dark:text-gray-200">Deploy flags & resource limits</h3>
				<ul class="mb-4 space-y-1 text-sm text-gray-600 dark:text-gray-300">
					<li><code class="rounded bg-gray-100 px-1 dark:bg-gray-700">--reset-db</code> — wipes <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">mysql-data</code> before starting MySQL (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">deploy.sh</code> only).</li>
					<li><code class="rounded bg-gray-100 px-1 dark:bg-gray-700">--keep-data</code> / <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">--wipe-data</code> — controls wiping store, cache, backup, and log volumes.</li>
					<li><code class="rounded bg-gray-100 px-1 dark:bg-gray-700">--clean-slate</code> — interactive full wipe of both database and data volumes (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">deploy.sh</code> only; requires typed confirmation).</li>
					<li><code class="rounded bg-gray-100 px-1 dark:bg-gray-700">--app-only</code> — swaps backend and frontend containers in place without restarting MySQL or Prometheus.</li>
					<li><code class="rounded bg-gray-100 px-1 dark:bg-gray-700">DEWEY_POD_CPUS</code> / <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">DEWEY_POD_MEMORY</code> — environment variable overrides for pod resource limits (defaults: <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">4</code> CPUs, <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">4g</code> memory).</li>
				</ul>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					Bundled <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">run.sh</code> supports <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">--keep-data</code>, <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">--wipe-data</code>, and <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">--app-only</code>, but neither <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">--reset-db</code> nor <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">--clean-slate</code> has a <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">run.sh</code> equivalent because it never touches <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">mysql-data</code>. Wiping the database on a bundled deployment requires running <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">podman volume rm mysql-data</code> manually.
				</p>

				<h3 class="mb-2 mt-4 text-sm font-semibold text-gray-700 dark:text-gray-200">Manual build — backend</h3>
				<pre class="overflow-x-auto rounded-lg bg-gray-50 p-4 text-xs dark:bg-gray-900"><code class="text-gray-800 dark:text-gray-200">podman build \
  --build-arg CGO_CFLAGS="-Wno-discarded-qualifiers" \
  -t dewey-backend ./backend

podman run -d --pod dewey-pod \
  -e DB_DSN="user:pass@tcp(host:3306)/deweyRecords" \
  --name dewey-backend dewey-backend</code></pre>

				<h3 class="mb-2 mt-4 text-sm font-semibold text-gray-700 dark:text-gray-200">Manual build — frontend</h3>
				<pre class="overflow-x-auto rounded-lg bg-gray-50 p-4 text-xs dark:bg-gray-900"><code class="text-gray-800 dark:text-gray-200">podman build -t dewey-frontend ./frontend/doctooladmin
podman run -d --pod dewey-pod --name dewey-frontend dewey-frontend</code></pre>

				<p class="mt-4 text-sm text-gray-600 dark:text-gray-300">
					The backend binary expects <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">./plugins</code>,
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">./cache</code>, <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">./store</code>,
					and <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">./backup</code> to be writable.
					Mount these as volumes if you need data to persist across container restarts.
				</p>
			</section>

			<!-- ── Troubleshooting ── -->
			<section id="troubleshooting" transition:fade={{ duration: 300 }} class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Troubleshooting</h2>
				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Deployment & Networking Gotchas</h3>
				<div class="space-y-4 text-sm text-gray-600 dark:text-gray-300">
					<div>
						<p class="font-semibold text-gray-800 dark:text-gray-200">1. HOST_IP fallback to literal "localhost"</p>
						<p class="mt-1">
							When <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">hostname -I</code> returns nothing (e.g. an isolated host or CI container),
							<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">deploy.sh</code> defaults <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">HOST_IP</code> to the literal string
							<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">"localhost"</code> and inserts it into <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">known_machines</code>.
							Because client requests are evaluated against parsed numeric IP addresses (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">127.0.0.1</code> or <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">::1</code>),
							<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">"localhost"</code> is a dead allowlist entry that will never match incoming traffic.
							Register the host's actual IP address or loopback (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">127.0.0.1</code>) in <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">known_machines</code> manually.
						</p>
					</div>
					<div>
						<p class="font-semibold text-gray-800 dark:text-gray-200">2. Podman NAT source-IP mismatch</p>
						<p class="mt-1">
							Rootless Podman networking backends (such as netavark or pasta) can translate connections from the host to published container ports under an internal bridge or gateway IP rather than <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">HOST_IP</code>.
							If health checks or client requests return <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">403 Forbidden</code>, inspect the backend logs:
						</p>
						<pre class="my-2 overflow-x-auto rounded-lg bg-gray-50 p-3 text-xs dark:bg-gray-900"><code class="text-gray-800 dark:text-gray-200">podman logs cross-doc-tool-dev</code></pre>
						<p>
							Look for the "unregistered machine" log line to find the exact source IP received by the server, and register that address via <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">dewey-cli add_machine &lt;ip&gt; &lt;label&gt;</code>.
						</p>
					</div>
					<div>
						<p class="font-semibold text-gray-800 dark:text-gray-200">3. IPv4 vs. IPv6 localhost resolution</p>
						<p class="mt-1">
							On dual-stack operating systems, connecting to <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">http://localhost:8080</code> may resolve to IPv6 loopback (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">::1</code>) before trying IPv4 (<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">127.0.0.1</code>).
							If only <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">127.0.0.1</code> is allowlisted, requests to <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">localhost</code> will be blocked with <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">403 Forbidden</code>.
							Ensure both <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">127.0.0.1</code> and <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">::1</code> are present in <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">known_machines</code>, or explicitly connect to <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">http://127.0.0.1:8080</code>.
						</p>
					</div>
				</div>
			</section>

		</main>

		<ScrollToTop />
	</div>
</div>
