<script>
	let activeStage = 'all'; // 'all', 'build', or 'production'

	const buildSteps = [
		{
			title: '1. Environment & Dependencies',
			desc: 'Sets up the Golang alpine image, configures the working directory, and caches Go modules to optimize subsequent build times.',
			code: `FROM golang:1.26-alpine AS builder\nWORKDIR /app\nRUN apk add --no-cache git # TODO: Check if we need this installed\n\nCOPY go.mod go.sum ./\nRUN go mod download`
		},
		{
			title: '2. Copy Source & Build',
			desc: 'Copies the application source code into the container and compiles the Go binary with specific compiler flags.',
			code: `COPY . .\n\n# Build binary\nARG CGO_CFLAGS="-Wno-discarded-qualifiers"\nENV CGO_CFLAGS=\${CGO_CFLAGS}\nRUN go build -o app-binary`
		}
	];

	const productionSteps = [
		{
			title: '1. Base Runtime Image',
			desc: 'Initializes a fresh, lightweight Alpine Linux environment to keep the final production image size as small as possible.',
			code: `FROM alpine:latest\nWORKDIR /app`
		},
		{
			title: '2. System Utilities',
			desc: 'Installs essential debugging and monitoring tools directly into the production container.',
			code: `# Install nano here\nRUN apk add --no-cache nano\nRUN apk add --no-cache curl\nRUN apk add --no-cache htop`
		},
		{
			title: '3. Artifact Extraction',
			desc: 'Crucial step for multi-stage builds: selectively copies only the compiled binary and necessary runtime assets from the builder stage, leaving behind the Go SDK and source files.',
			code: `# Copy only the binary from builder\nCOPY --from=builder /app/app-binary .\nCOPY --from=builder /app/templates ./templates\nCOPY --from=builder /app/rules ./rules`
		},
		{
			title: '4. Port Expose & Execution',
			desc: 'Documents the network port the container listens on at runtime and defines the default command to start the backend application.',
			code: `EXPOSE 8080\nCMD ["./app-binary"]`
		}
	];
</script>

<div class="min-h-screen bg-slate-900 p-6 font-sans text-slate-100 md:p-12">
	<div class="mx-auto max-w-4xl">
		<header class="mb-8 border-b border-slate-800 pb-6">
			<div class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
				<div>
					<span
						class="rounded-full bg-indigo-500/10 px-3 py-1 text-xs font-semibold tracking-wider text-indigo-400 uppercase"
					>
						Podman Build Deployment File
					</span>
					<h1 class="mt-3 text-3xl font-bold text-white">Backend Containerization</h1>
					<p class="mt-1 flex items-center gap-2 text-sm text-slate-400">
						<span class="rounded bg-slate-800 px-2 py-0.5 font-mono text-indigo-300"
							>./backend/Dockerfile</span
						>
					</p>
				</div>

				<div class="flex self-start rounded-lg bg-slate-800 p-1 md:self-center">
					<button
						class="rounded-md px-4 py-1.5 text-sm font-medium transition-colors {activeStage ===
						'all'
							? 'bg-indigo-600 text-white'
							: 'text-slate-400 hover:text-slate-200'}"
						on:click={() => (activeStage = 'all')}
					>
						All
					</button>
					<button
						class="rounded-md px-4 py-1.5 text-sm font-medium transition-colors {activeStage ===
						'build'
							? 'bg-indigo-600 text-white'
							: 'text-slate-400 hover:text-slate-200'}"
						on:click={() => (activeStage = 'build')}
					>
						Build Stage
					</button>
					<button
						class="rounded-md px-4 py-1.5 text-sm font-medium transition-colors {activeStage ===
						'production'
							? 'bg-indigo-600 text-white'
							: 'text-slate-400 hover:text-slate-200'}"
						on:click={() => (activeStage = 'production')}
					>
						Production Stage
					</button>
				</div>
			</div>

			<p class="mt-4 max-w-2xl text-sm leading-relaxed text-slate-400">
				This multi-stage Dockerfile is used with Podman to build and deploy the backend application.
			</p>
		</header>

		<div class="space-y-12">
			{#if activeStage === 'all' || activeStage === 'build'}
				<section class="relative space-y-6 border-l-2 border-indigo-500/30 pl-6">
					<div
						class="absolute top-0 -left-[9px] h-4 w-4 rounded-full bg-indigo-500 ring-4 ring-indigo-950"
					></div>

					<div class="mb-4">
						<h2
							class="flex items-center gap-2 text-xl font-bold tracking-wide text-indigo-400 text-white uppercase"
						>
							Build Stage
							<span class="text-xs font-normal text-slate-500 normal-case">AS builder</span>
						</h2>
						<p class="text-xs text-slate-400">
							Compiles application source binaries safely away from production environments.
						</p>
					</div>

					{#each buildSteps as step}
						<div
							class="rounded-xl border border-slate-800 bg-slate-800/50 p-5 shadow-sm transition-all hover:border-slate-700/70"
						>
							<h3 class="text-md mb-2 font-semibold text-slate-200">{step.title}</h3>
							<p class="mb-4 text-sm leading-relaxed font-light text-slate-400">{step.desc}</p>
							<pre
								class="overflow-x-auto rounded-lg border border-slate-900/60 bg-slate-950 p-4 font-mono text-xs text-emerald-400"><code
									>{step.code}</code
								></pre>
						</div>
					{/each}
				</section>
			{/if}

			{#if activeStage === 'all' || activeStage === 'production'}
				<section class="relative space-y-6 border-l-2 border-emerald-500/30 pl-6">
					<div
						class="absolute top-0 -left-[9px] h-4 w-4 rounded-full bg-emerald-500 ring-4 ring-emerald-950"
					></div>

					<div class="mb-4">
						<h2
							class="flex items-center gap-2 text-xl font-bold tracking-wide text-emerald-400 text-white uppercase"
						>
							Production Stage
							<span class="text-xs font-normal text-slate-500 normal-case">alpine:latest</span>
						</h2>
						<p class="text-xs text-slate-400">
							Minimalistic, secure runner layer that executes the pre-compiled binary.
						</p>
					</div>

					{#each productionSteps as step}
						<div
							class="rounded-xl border border-slate-800 bg-slate-800/50 p-5 shadow-sm transition-all hover:border-slate-700/70"
						>
							<h3 class="text-md mb-2 font-semibold text-slate-200">{step.title}</h3>
							<p class="mb-4 text-sm leading-relaxed font-light text-slate-400">{step.desc}</p>
							<pre
								class="overflow-x-auto rounded-lg border border-slate-900/60 bg-slate-950 p-4 font-mono text-xs text-emerald-400"><code
									>{step.code}</code
								></pre>
						</div>
					{/each}
				</section>
			{/if}
		</div>

		<footer class="mt-16 border-t border-slate-800/60 pt-6 text-center text-xs text-slate-500">
			Podman Blueprint Pipeline Layout • Optimized for Svelte & Tailwind CSS
		</footer>
	</div>
</div>
