<script lang="ts">
    import Sidebar from '$lib/components/Sidebar.svelte';
    import Topbar from '$lib/components/Topbar.svelte';

    let sidebarOpen = true;

    function toggleSidebar() {
        sidebarOpen = !sidebarOpen;
    }
</script>

<div class="min-h-screen bg-gray-100 flex">
    <!-- Sidebar Component -->
	<Sidebar open={sidebarOpen} toggle={toggleSidebar} />

	<!-- Main Content -->
	<div class="flex-1 flex flex-col">
		<!-- Topbar -->
		<Topbar {sidebarOpen} {toggleSidebar} />
    </div>
</div>