<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Topbar from '$lib/components/Topbar.svelte';
  	import { Table, TableBody, TableBodyCell, TableBodyRow, TableHead, TableHeadCell, Checkbox, TableSearch } from "flowbite-svelte";

	let sidebarOpen = $state(true);

	const toggleSidebar = () => {
		sidebarOpen = !sidebarOpen;
	};
</script>

<div class="min-h-screen bg-gray-100 flex">
    <!-- Sidebar Component -->
	<Sidebar open={sidebarOpen} toggle={toggleSidebar} />

	<!-- Main Content -->
	<div class="flex-1 flex flex-col">
		<!-- Topbar -->
		<Topbar {sidebarOpen} {toggleSidebar} />

		<!-- Content Grid -->
		<main class="p-6">
			<div
				class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6"
			>
				<!-- Card -->
				<div class="bg-white p-4 rounded-2xl shadow-sm">
					<h2 class="text-lg font-semibold mb-2">File</h2>
					<p class="text-gray-600 text-sm">Some content here</p>
				</div>

				<div class="bg-white p-4 rounded-2xl shadow-sm">
					<h2 class="text-lg font-semibold mb-2">Card 2</h2>
					<p class="text-gray-600 text-sm">Some content here</p>
				</div>

				<div class="bg-white p-4 rounded-2xl shadow-sm">
					<h2 class="text-lg font-semibold mb-2">Card 3</h2>
					<p class="text-gray-600 text-sm">Some content here</p>
				</div>

				<div class="bg-white p-4 rounded-2xl shadow-sm">
					<h2 class="text-lg font-semibold mb-2">Card 4</h2>
					<p class="text-gray-600 text-sm">Some content here</p>
				</div>

				<!-- Wide Card -->
				<div class="bg-white p-4 rounded-2xl shadow-sm col-span-1 sm:col-span-2 lg:col-span-4">
					<h2 class="text-lg font-semibold mb-2">Wide Section</h2>
					<p class="text-gray-600 text-sm">
						Useful for charts, tables, logs, etc.
					</p>
				</div>
			</div>
		</main>

		<div class="p-4 text-center text-sm m-4 rounded-lg">
		<Table>
			<TableHead>
				<TableHeadCell>Product name</TableHeadCell>
				<TableHeadCell>Color</TableHeadCell>
				<TableHeadCell>Category</TableHeadCell>
				<TableHeadCell>Price</TableHeadCell>
			</TableHead>
			<TableBody>
				<TableBodyRow>
					<TableBodyCell>Apple MacBook Pro 17"</TableBodyCell>
					<TableBodyCell>Silver</TableBodyCell>
					<TableBodyCell>Laptop</TableBodyCell>
					<TableBodyCell>$2999</TableBodyCell>
				</TableBodyRow>
				<TableBodyRow>
					<TableBodyCell>Microsoft Surface Pro</TableBodyCell>
					<TableBodyCell>White</TableBodyCell>
					<TableBodyCell>Laptop PC</TableBodyCell>
					<TableBodyCell>$1999</TableBodyCell>
				</TableBodyRow>
				<TableBodyRow>
					<TableBodyCell>Magic Mouse 2</TableBodyCell>
					<TableBodyCell>Black</TableBodyCell>
					<TableBodyCell>Accessories</TableBodyCell>
					<TableBodyCell>$99</TableBodyCell>
				</TableBodyRow>
			</TableBody>
		</Table>
		</div>
	</div>
</div>