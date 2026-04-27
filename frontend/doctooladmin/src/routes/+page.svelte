<script lang="ts">
	let email = '';
	let password = '';

	function handleLogin() {
		console.log('Email:', email);
		console.log('Password:', password);
		// TODO: hook into your auth logic
	}
</script>

<div class="min-h-screen flex items-center justify-center bg-surface-100">
	<div class="card w-full max-w-md p-6 space-y-6">
		<h1 class="text-2xl font-bold text-center">Login</h1>

		<form class="space-y-4" on:submit|preventDefault={handleLogin}>
			<div>
				<label class="label">
					<span>Email</span>
				</label>
				<input
					class="input"
					type="email"
					placeholder="you@example.com"
					bind:value={email}
					required
				/>
			</div>

			<div>
				<label class="label">
					<span>Password</span>
				</label>
				<input
					class="input"
					type="password"
					placeholder="••••••••"
					bind:value={password}
					required
				/>
			</div>

			<button
				type="submit"
				class="btn preset-filled-primary w-full"
			>
				Sign In
			</button>
		</form>

		<p class="text-sm text-center">
			Don’t have an account?
			<a href="/register" class="anchor">Sign up</a>
		</p>
	</div>
</div>
