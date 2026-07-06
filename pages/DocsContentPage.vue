<template>
    <div class="doc-portal min-h-screen bg-white font-urbanist selection:bg-[#ff6b00]/20 selection:text-[#ff6b00]">
        <PublicNavbar activeLink="docs" />
        <div class="h-20"></div>
        <div class="flex h-[calc(100vh-80px)]">
            <!-- Unified Multi-Service Sidebar -->
            <DocSidebar :manifests="docsStore.manifests"
                :loading="docsStore.loading && Object.keys(docsStore.manifests).length === 0" />

            <!-- Main Content -->
            <main class="flex-1 flex flex-col overflow-hidden relative">

                <!-- Global Navigation Overlay (Fixed Top) -->
                <header
                    class="h-16 border-b border-gray-100 flex items-center justify-between px-10 bg-white/90 backdrop-blur-md z-20">
                    <div class="flex items-center gap-5">
                        <div v-if="docsStore.currentDoc?.metadata.icon"
                            class="w-10 h-10 bg-gray-50 flex items-center justify-center text-[#ff6b00] border border-gray-100">
                            <component :is="getIcon(docsStore.currentDoc.metadata.icon)" :size="20" />
                        </div>
                        <div>
                            <div
                                class="flex items-center gap-2 text-[10px] font-black text-gray-400 uppercase tracking-widest mb-0.5 italic">
                                <span>{{ route.params.service }}</span>
                                <span v-if="docsStore.currentDoc?.metadata.lastUpdated">/ UPDATE: {{
                                    docsStore.currentDoc.metadata.lastUpdated }}</span>
                            </div>
                            <h1 class="text-xl font-black text-[#232f3e] tracking-tighter uppercase leading-none">
                                {{ docsStore.currentDoc?.metadata.title || 'Loading Documentation...' }}
                            </h1>
                        </div>
                    </div>

                    <div class="flex items-center gap-8">
                        <div class="hidden md:flex items-center gap-2">
                            <span v-for="tag in docsStore.currentDoc?.metadata.tags" :key="tag"
                                class="px-2.5 py-1 bg-white border border-gray-100 text-[9px] font-black text-[#879196] uppercase tracking-[0.2em] italic">
                                {{ tag }}
                            </span>
                        </div>
                        <button @click="router.back()"
                            class="p-2.5 hover:bg-[#232f3e] hover:text-white border border-transparent transition-all duration-300">
                            <X :size="20" />
                        </button>
                    </div>
                </header>

                <!-- Scrollable Content View -->
                <div class="flex-1 overflow-y-auto custom-scrollbar bg-[#fafafa]">
                    <div
                        class="max-w-5xl mx-auto py-24 px-16 bg-white min-h-full border-x border-gray-100 shadow-[0_0_80px_rgba(0,0,0,0.015)]">

                        <!-- Loading Overlay -->
                        <div v-if="docsStore.loading && !docsStore.currentDoc"
                            class="py-32 flex flex-col items-center justify-center space-y-8 animate-pulse">
                            <div class="w-16 h-16 bg-gray-50 border-2 border-dashed border-gray-200"></div>
                            <div class="space-y-3 flex flex-col items-center">
                                <div class="h-4 bg-gray-50 rounded w-64"></div>
                                <div class="h-3 bg-gray-50 rounded w-40"></div>
                            </div>
                        </div>

                        <!-- Error State -->
                        <div v-else-if="docsStore.error"
                            class="py-32 flex flex-col items-center justify-center text-center">
                            <div
                                class="w-20 h-20 bg-red-50 flex items-center justify-center mb-8 border border-red-100">
                                <AlertCircle class="text-red-500" :size="40" />
                            </div>
                            <h2 class="text-3xl font-black text-[#232f3e] tracking-tighter uppercase mb-4 italic">
                                Resource
                                Missing</h2>
                            <p class="text-[#545b64] max-w-md font-medium mb-10 leading-relaxed">{{ docsStore.error }}
                            </p>
                            <button @click="fetchInitialData"
                                class="px-10 py-4 bg-[#232f3e] text-white text-xs font-black uppercase tracking-widest hover:bg-[#ff6b00] transition-colors">
                                INIT RELOAD PROTOCOL
                            </button>
                        </div>

                        <!-- Active Content -->
                        <div v-else-if="docsStore.currentDoc" class="fade-in">
                            <div class="mb-16 pb-12 border-b border-gray-50">
                                <p
                                    class="text-2xl text-[#879196] leading-relaxed font-medium tracking-tight italic max-w-3xl">
                                    {{ docsStore.currentDoc.metadata.description }}
                                </p>
                            </div>

                            <MarkdownRenderer :content="docsStore.currentDoc.content"
                                :service="route.params.service as string" />
                        </div>

                        <!-- Empty State -->
                        <div v-else
                            class="py-32 flex flex-col items-center justify-center text-gray-400 opacity-20 group">
                            <BookOpen :size="100"
                                class="mb-8 group-hover:scale-110 transition-transform duration-700" />
                            <p class="tracking-[0.4em] font-black uppercase text-xs">AWAITING INPUT</p>
                        </div>
                    </div>

                    <!-- Strategic Footer Padding -->
                    <div class="h-40"></div>
                </div>
            </main>
        </div>
    </div>
</template>

<script setup lang="ts">
import { onMounted, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { X, BookOpen, AlertCircle, Database, Shield, Settings, BarChart3, HelpCircle, Server, Cpu, Layers } from 'lucide-vue-next';
import { useDocsStore } from '../store/docsStore';
import DocSidebar from '../components/DocSidebar.vue';
import MarkdownRenderer from '../components/MarkdownRenderer.vue';
import PublicNavbar from '@/shared/components/PublicNavbar.vue';

const docsStore = useDocsStore();
const route = useRoute();
const router = useRouter();

const fetchInitialData = async () => {
    if (Object.keys(docsStore.manifests).length === 0) {
        await docsStore.fetchAllManifests();
    }

    const { service, slug } = route.params;

    if (service && slug) {
        await docsStore.fetchDocContent(service as string, slug as string);
    } else if (service) {
        const manifest = docsStore.manifests[service as string];
        if (manifest?.categories[0]?.items[0]) {
            router.replace({
                name: 'docs-content',
                params: { service, slug: manifest.categories[0].items[0].slug }
            });
        } else {
            redirectToFirstAvailableDoc();
        }
    } else {
        redirectToFirstAvailableDoc();
    }
};

const redirectToFirstAvailableDoc = () => {
    const manifests = docsStore.manifests;

    for (const service of Object.keys(manifests)) {
        const firstItem = manifests[service]?.categories?.[0]?.items?.[0];
        if (firstItem) {
            router.replace({
                name: 'docs-content',
                params: { service, slug: firstItem.slug }
            });
            return;
        }
    }

    console.warn('No docs available across any service');
};

onMounted(fetchInitialData);

watch(() => [route.params.service, route.params.slug], ([service, slug]) => {
    if (service && slug) {
        docsStore.fetchDocContent(service as string, slug as string);
    }
});

const getIcon = (iconName: string) => {
    const icons: Record<string, any> = {
        database: Database,
        shield: Shield,
        settings: Settings,
        analytics: BarChart3,
        help: HelpCircle,
        book: BookOpen,
        server: Server,
        cpu: Cpu,
        layers: Layers
    };
    return icons[iconName.toLowerCase()] || BookOpen;
};
</script>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Urbanist:wght@300;400;500;600;700;800;900&display=swap');

.font-urbanist {
    font-family: 'Urbanist', sans-serif;
}

.custom-scrollbar::-webkit-scrollbar {
    width: 6px;
}

.custom-scrollbar::-webkit-scrollbar-track {
    @apply bg-transparent;
}

.custom-scrollbar::-webkit-scrollbar-thumb {
    @apply bg-gray-200 rounded-full;
}

.custom-scrollbar::-webkit-scrollbar-thumb:hover {
    @apply bg-gray-300;
}

.fade-in {
    animation: fadeIn 0.6s cubic-bezier(0.16, 1, 0.3, 1) forwards;
}

@keyframes fadeIn {
    from {
        opacity: 0;
        transform: translateY(20px);
    }

    to {
        opacity: 1;
        transform: translateY(0);
    }
}
</style>
