<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Topbar from '$lib/components/Topbar.svelte';
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
		{ href: '#uploading', label: 'Uploading Files' },
		{ href: '#retrieving', label: 'Retrieving Files' },
		{ href: '#network', label: 'Known Machines' },
		{ href: '#analytics', label: 'Analytics' },
		{ href: '#settings', label: 'Settings' },
		{ href: '#system', label: 'System Config' },
		{ href: '#troubleshooting', label: 'Troubleshooting' }
	];
</script>

<svelte:head>
	<title>Admin Guide — Dewey</title>
</svelte:head>

<div class="flex min-h-screen bg-gray-100 dark:bg-gray-900">
	<Sidebar open={sidebarOpen} toggle={toggleSidebar} />

	<div class="flex flex-1 flex-col">
		<Topbar {sidebarOpen} {toggleSidebar} />

		<main id="main-content" class={mainClass}>

			<!-- Header -->
			<div>
				<p class="text-sm text-gray-500 dark:text-gray-400">
					<a href="/docs" class="hover:underline">Documentation</a> / Admin Guide
				</p>
				<h1 class="mt-1 text-2xl font-bold text-gray-800 dark:text-white">Admin Guide</h1>
				<p class="text-sm text-gray-500 dark:text-gray-400">
					Day-to-day operation, file management, settings, and monitoring.
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
			<section id="overview" class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Overview</h2>
				<p class="mb-3 text-sm text-gray-600 dark:text-gray-300">
					Dewey is a document management and file sorting system. When a file is submitted, Dewey
					validates it, scans it for barcodes, sorts it into the right location using configurable
					rules (plugins), and stores it alongside its metadata. Files can then be retrieved or
					deleted through the API.
				</p>
				<p class="mb-3 text-sm text-gray-600 dark:text-gray-300">
					This portal gives you visibility into the system's health, lets you tune its behaviour,
					and is the primary place to monitor activity.
				</p>
				<div class="grid gap-3 sm:grid-cols-3">
					<div class="rounded-lg bg-gray-50 p-3 dark:bg-gray-700">
						<p class="mb-1 text-xs font-semibold text-gray-700 dark:text-gray-200">Backend API</p>
						<p class="text-xs text-gray-500 dark:text-gray-400">Port 8080 — handles all file operations and system control.</p>
					</div>
					<div class="rounded-lg bg-gray-50 p-3 dark:bg-gray-700">
						<p class="mb-1 text-xs font-semibold text-gray-700 dark:text-gray-200">Admin Portal</p>
						<p class="text-xs text-gray-500 dark:text-gray-400">Port 3000 — this interface. Analytics, settings, and docs.</p>
					</div>
					<div class="rounded-lg bg-gray-50 p-3 dark:bg-gray-700">
						<p class="mb-1 text-xs font-semibold text-gray-700 dark:text-gray-200">Prometheus</p>
						<p class="text-xs text-gray-500 dark:text-gray-400">Port 9090 — raw metrics if you need deeper inspection.</p>
					</div>
				</div>
			</section>

			<!-- ── Uploading Files ── -->
			<section id="uploading" class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Uploading Files</h2>

				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					Files are submitted via a <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">POST</code> to
					<code class="rounded bg-gray-100 px-1 dark:bg-gray-700">http://&lt;host&gt;:8080/upload</code>
					as <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">multipart/form-data</code>.
					This is typically done by your document submission tool or integration, not manually.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Allowed File Types</h3>
				<p class="mb-2 text-sm text-gray-600 dark:text-gray-300">
					Only the following extensions are accepted. All others are rejected before any processing occurs.
				</p>
				<div class="mb-4 flex flex-wrap gap-2">
					{#each ['.pdf', '.txt', '.doc', '.docx', '.xls', '.xlsx', '.csv', '.ppt', '.png', '.jpg', '.jpeg'] as ext}
						<span class="rounded bg-gray-100 px-2 py-0.5 font-mono text-xs dark:bg-gray-700 dark:text-gray-300">{ext}</span>
					{/each}
				</div>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">File Size Limit</h3>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					The default maximum file size is <strong class="text-gray-700 dark:text-gray-200">50 MB</strong>.
					This can be changed by an administrator in Settings → System → Max Upload File Size.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Security Checks</h3>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					Every file — regardless of extension — is inspected for Windows PE and Linux ELF executable
					signatures. Any file that appears to be an executable binary is rejected, even if it has
					a permitted extension like <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">.pdf</code>.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">What happens after upload</h3>
				<ol class="space-y-1 text-sm text-gray-600 dark:text-gray-300">
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">1</span><span>File is placed in the cache temporarily.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">2</span><span>Any barcode or QR code in the file is decoded and recorded.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">3</span><span>Sorting rules (plugins) run and determine the file's destination folder.</span></li>
					<li class="flex gap-3"><span class="font-mono text-xs font-bold text-gray-400">4</span><span>File is moved to permanent storage and a database record is created.</span></li>
				</ol>

				<div class="mt-4 rounded-lg border border-yellow-200 bg-yellow-50 p-3 text-xs text-yellow-800 dark:border-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-300">
					<strong>Note:</strong> The cache is cleared periodically by the background daemon. Do not
					rely on files remaining in the cache — always retrieve from the permanent store via
					the filename.
				</div>
			</section>

			<!-- ── Retrieving Files ── -->
			<section id="retrieving" class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Retrieving Files</h2>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">List all files</h3>
				<p class="mb-2 text-sm text-gray-600 dark:text-gray-300">Returns a JSON list of filenames currently in the cache.</p>
				<pre class="mb-4 rounded-lg bg-gray-50 p-3 text-xs dark:bg-gray-900"><code class="text-gray-800 dark:text-gray-200">GET http://&lt;host&gt;:8080/files</code></pre>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Get a specific file</h3>
				<p class="mb-2 text-sm text-gray-600 dark:text-gray-300">
					Returns the file as a stream. Use the timestamped filename returned at upload time.
				</p>
				<pre class="mb-4 rounded-lg bg-gray-50 p-3 text-xs dark:bg-gray-900"><code class="text-gray-800 dark:text-gray-200">GET http://&lt;host&gt;:8080/files/&lt;filename&gt;/false</code></pre>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Delete a file</h3>
				<p class="mb-2 text-sm text-gray-600 dark:text-gray-300">
					Removes the file from the cache. The database record is soft-deleted (not permanently removed).
				</p>
				<pre class="mb-4 rounded-lg bg-gray-50 p-3 text-xs dark:bg-gray-900"><code class="text-gray-800 dark:text-gray-200">DELETE http://&lt;host&gt;:8080/files/&lt;filename&gt;</code></pre>

				<div class="rounded-lg border border-blue-200 bg-blue-50 p-3 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-300">
					<strong>Filenames:</strong> Dewey prefixes every uploaded file with a Unix timestamp to prevent
					collisions, e.g. <code>1700000000_report.pdf</code>. The original filename is preserved after
					the prefix. The full timestamped name is returned in the upload response and should be stored
					by your integration.
				</div>
			</section>

			<!-- ── Known Machines ── -->
			<section id="network" class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Known Machines</h2>
				<p class="mb-3 text-sm text-gray-600 dark:text-gray-300">
					Dewey only accepts requests from machines whose IP address has been registered ahead of
					time. Anything else — including the file manager portal, if opened from an unregistered
					machine — gets rejected with <code class="rounded bg-gray-100 px-1 dark:bg-gray-700">403 Forbidden</code>
					before it reaches any file operation.
				</p>
				<p class="mb-3 text-sm text-gray-600 dark:text-gray-300">
					Use the <strong class="text-gray-700 dark:text-gray-200">Known Machines</strong> page in the
					sidebar to see every registered machine, when it was added, and when it last connected —
					and to add or remove machines yourself.
				</p>
				<div class="rounded-lg border border-yellow-200 bg-yellow-50 p-3 text-xs text-yellow-800 dark:border-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-300">
					<strong>Note:</strong> you can only reach the Known Machines page from a machine that's
					already registered — a brand-new machine can't add itself. The very first entry has to be
					set up directly against the database by whoever deployed the system.
				</div>
			</section>

			<!-- ── Analytics ── -->
			<section id="analytics" class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Analytics</h2>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					The Analytics page shows live metrics pulled from the backend. They refresh at the interval
					configured in Settings → Behavior → Metrics Poll Interval.
				</p>

				<div class="space-y-3">
					{#each [
						{ name: 'Files Uploaded', desc: 'Total number of files successfully processed since the server started.' },
						{ name: 'File Sorts', desc: 'How many files have been moved from cache to permanent storage by the pipeline.' },
						{ name: 'File Retrievals', desc: 'Total number of times a file has been fetched via the API.' },
						{ name: 'File Deletions', desc: 'Total number of cache deletions triggered manually or by the API.' },
						{ name: 'File Copies', desc: 'Total copy operations from cache to store. Should equal File Sorts in normal operation.' },
						{ name: 'Barcode Successes / Failures', desc: 'Count of files where barcode scanning succeeded or failed. High failures may indicate image quality issues.' },
						{ name: 'Plugin Runs / Errors', desc: 'How many times plugins have executed, and how many errors were encountered. Plugin errors are non-fatal but the file may be sorted incorrectly.' },
						{ name: 'DB Errors', desc: 'Database write failures. Any non-zero value warrants investigation.' },
						{ name: 'Upload Rejections', desc: 'Files rejected before processing — broken down by reason (invalid extension, PE detected, ELF detected).' },
						{ name: 'Upload Size Distribution', desc: 'Histogram of uploaded file sizes. Useful for capacity planning.' },
					] as metric}
						<div class="flex gap-4">
							<p class="w-48 shrink-0 text-sm font-medium text-gray-700 dark:text-gray-200">{metric.name}</p>
							<p class="text-sm text-gray-500 dark:text-gray-400">{metric.desc}</p>
						</div>
					{/each}
				</div>

				<div class="mt-4 rounded-lg border border-blue-200 bg-blue-50 p-3 text-xs text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-300">
					All metrics shown here are <strong>in-memory counters</strong> that reset when the server
					restarts. For long-term historical data, use the Prometheus endpoint at port 9090.
				</div>
			</section>

			<!-- ── Settings ── -->
			<section id="settings" class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Settings</h2>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					Settings are accessed via the sidebar. They are saved immediately to your browser's local
					storage — no save button needed, and changes apply instantly. They are per-browser and
					do not affect other users.
				</p>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Behavior</h3>
				<div class="mb-4 space-y-2">
					{#each [
						{ name: 'Metrics Poll Interval', desc: 'How often the Analytics and Dashboard pages refresh. Lower values give more real-time data but increase backend traffic. Options: 1s, 5s, 15s, 30s.' },
						{ name: 'Analytics History Window', desc: 'How many data points the sparkline charts retain. More points = longer history visible, higher memory use in the browser.' },
						{ name: 'Prometheus Alert Banner', desc: 'When enabled, a red banner appears at the top of the page if the backend metrics endpoint cannot be reached. Disable if the Prometheus banner is distracting in stable environments.' },
						{ name: 'Default Sidebar State', desc: 'Whether the sidebar is open or collapsed when you first load the app on any page.' },
						{ name: 'Layout Density', desc: 'Comfortable adds more padding throughout the interface. Compact reduces spacing for smaller screens or denser information display.' },
					] as s}
						<div class="flex gap-4">
							<p class="w-52 shrink-0 text-sm font-medium text-gray-700 dark:text-gray-200">{s.name}</p>
							<p class="text-sm text-gray-500 dark:text-gray-400">{s.desc}</p>
						</div>
					{/each}
				</div>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Alerts</h3>
				<div class="mb-4 space-y-2">
					{#each [
						{ name: 'RAM Alert Threshold', desc: 'Shows a warning on the dashboard when backend RAM usage exceeds this value in megabytes. Default is 200 MB.' },
						{ name: 'Retry Alert Threshold', desc: 'Shows a warning when file processing retries exceed this count. A non-zero retry count usually indicates a transient failure in the pipeline.' },
					] as s}
						<div class="flex gap-4">
							<p class="w-52 shrink-0 text-sm font-medium text-gray-700 dark:text-gray-200">{s.name}</p>
							<p class="text-sm text-gray-500 dark:text-gray-400">{s.desc}</p>
						</div>
					{/each}
				</div>

				<h3 class="mb-2 text-sm font-semibold text-gray-700 dark:text-gray-200">Customization</h3>
				<div class="space-y-2">
					{#each [
						{ name: 'Dark Mode', desc: 'Switches the entire portal to a dark colour scheme. Saved per browser.' },
						{ name: 'Accent Color', desc: 'The highlight colour used on cards, active sidebar links, and section headings.' },
						{ name: 'Chart Color Theme', desc: 'The colour palette used by all analytics charts. Default, Cool, Warm, and Mono are available.' },
					] as s}
						<div class="flex gap-4">
							<p class="w-52 shrink-0 text-sm font-medium text-gray-700 dark:text-gray-200">{s.name}</p>
							<p class="text-sm text-gray-500 dark:text-gray-400">{s.desc}</p>
						</div>
					{/each}
				</div>
			</section>

			<!-- ── System Config ── -->
			<section id="system" class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">System Configuration</h2>
				<p class="mb-4 text-sm text-gray-600 dark:text-gray-300">
					The System section of Settings controls backend behaviour. Changes here are sent to the
					server via the admin API and take effect immediately without a restart.
				</p>
				<div class="space-y-2">
					{#each [
						{ name: 'Daemon Tick Interval', desc: 'How often (in hours) the background daemon runs. On each cycle it clears the file cache and creates a backup of permanent storage. Reducing this means more frequent backups and more aggressive cache clearing. Default: 1 hour.' },
						{ name: 'Max Upload File Size', desc: 'The largest file the server will accept in a single upload request, in megabytes. Files exceeding this are rejected before any processing. Default: 50 MB.' },
						{ name: 'Max Open DB Connections', desc: 'The maximum number of simultaneous open connections to the MySQL database. Increase if you see database timeout errors under heavy load. Default: 10.' },
						{ name: 'Max Idle DB Connections', desc: 'The number of connections kept ready in the pool when not in use. Higher values reduce connection setup latency at the cost of held resources. Default: 10.' },
						{ name: 'DB Connection Timeout', desc: 'How long (in minutes) a database connection is kept alive before being recycled. Recycling connections helps recover from silent disconnects. Default: 2 minutes.' },
					] as s}
						<div class="flex gap-4">
							<p class="w-52 shrink-0 text-sm font-medium text-gray-700 dark:text-gray-200">{s.name}</p>
							<p class="text-sm text-gray-500 dark:text-gray-400">{s.desc}</p>
						</div>
					{/each}
				</div>

				<div class="mt-4 rounded-lg border border-yellow-200 bg-yellow-50 p-3 text-xs text-yellow-800 dark:border-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-300">
					<strong>Note:</strong> System settings are applied to the running server immediately but are
					not persisted to disk. If the server restarts, it will revert to the compiled-in defaults.
					To make changes permanent, update <code>consts.go</code> and redeploy.
				</div>
			</section>

			<!-- ── Troubleshooting ── -->
			<section id="troubleshooting" class="scroll-mt-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">Troubleshooting</h2>

				<div class="space-y-5">
					{#each [
						{
							problem: 'Prometheus banner / metrics not loading',
							causes: [
								'The backend server is not running.',
								'The server is running but the metrics endpoint (port 8080) is not reachable from the frontend container.',
								'The Prometheus scraper on port 9090 is down (this does not affect the API metrics endpoint).',
							],
							fix: 'Check that the backend container is running: podman ps. Verify the pod network is intact. Review the backend logs for Fatal entries.'
						},
						{
							problem: 'File upload rejected — "Invalid file type"',
							causes: [
								'The file extension is not in the allowlist.',
								'The file has a permitted extension but its internal bytes look like an executable (PE or ELF header detected).',
							],
							fix: 'Verify the file extension. If the file is legitimate but being flagged as an executable, it may be corrupted — re-export it from the source application.'
						},
						{
							problem: 'File uploaded but not appearing in retrieval',
							causes: [
								'The cache was cleared by the daemon between upload and retrieval (cache is ephemeral).',
								'The plugin chain set an unexpected destination path — the file is in permanent storage but under a different path than expected.',
							],
							fix: 'Use GET /files to list what is currently in cache. For permanent storage, check the database record directly — the filepath column holds the exact location within ./store.'
						},
						{
							problem: 'High DB Errors count on the Analytics page',
							causes: [
								'The MySQL server is unreachable or the connection pool is exhausted.',
								'The schema is out of date (migration did not run cleanly on the last start).',
							],
							fix: 'Check backend logs for WARN or FATAL lines containing "transaction" or "database". Verify the DB_DSN environment variable is correct and the database server is healthy.'
						},
						{
							problem: 'High Barcode Failures count',
							causes: [
								'Source documents do not contain barcodes — this is expected for many document types.',
								'Barcode images are too small, low resolution, or distorted.',
							],
							fix: 'If barcode scanning is not required for your workflow, barcode failures are informational only and do not affect sorting or storage. If barcodes are expected, check the source document quality.'
						},
						{
							problem: '403 Forbidden — "unregistered machine"',
							causes: [
								'The request came from an IP address that has not been added to the Known Machines list.',
								'The machine\'s IP changed (e.g. DHCP reassignment) since it was registered.',
							],
							fix: 'From an already-registered machine, open Known Machines in the sidebar and add the new IP with a label. If this is the very first machine being set up, it must be registered directly against the database instead.'
						},
						{
							problem: 'Plugin Errors showing in Analytics',
							causes: [
								'A Lua plugin script has a runtime error.',
								'A plugin\'s hook function (e.g. OnFilter, OnUpload) returned an unexpected type instead of the entry table.',
							],
							fix: 'Check the backend logs for WARN lines containing "plugin". The error message will include the plugin name and the Lua error. Fix the plugin script in ./plugins/ — it is reloaded on the next server start.'
						},
					] as item}
						<div>
							<p class="mb-1 text-sm font-semibold text-gray-700 dark:text-gray-200">{item.problem}</p>
							<p class="mb-1 text-xs font-medium text-gray-500 dark:text-gray-400">Possible causes:</p>
							<ul class="mb-2 space-y-0.5 pl-3 text-xs text-gray-600 dark:text-gray-400">
								{#each item.causes as cause}
									<li class="list-disc">{cause}</li>
								{/each}
							</ul>
							<p class="text-xs text-gray-600 dark:text-gray-400">
								<strong class="text-gray-700 dark:text-gray-300">Fix:</strong> {item.fix}
							</p>
						</div>
					{/each}
				</div>
			</section>

		</main>
	</div>
</div>
