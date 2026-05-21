<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import PublicNavbar from '@/shared/components/PublicNavbar.vue'

const router = useRouter()
const searchQuery = ref('')

const docs = []

const filteredDocs = computed(() => {
    if (!searchQuery.value) return docs
    const query = searchQuery.value.toLowerCase()
    return docs.filter(doc =>
        doc.title.toLowerCase().includes(query) ||
        doc.description.toLowerCase().includes(query) ||
        doc.tag.toLowerCase().includes(query)
    )
})

const navigateToDoc = (doc: any) => {
    if (doc.status === 'active') {
        router.push(doc.route)
    }
}

onMounted(() => {
    window.scrollTo(0, 0)
})
</script>

<template>
    <div class="min-h-screen bg-white text-[#16191f] font-urbanist selection:bg-[#ff9900]/30 selection:text-[#16191f]">
        <PublicNavbar activeLink="docs" />

        <!-- Spacer for fixed navbar -->
        <div class="h-20"></div>

        <!-- Hero Section -->
        <section class="py-24 bg-[#fafafa] border-b border-[#eaeded]">
            <div class="max-w-7xl mx-auto px-6">
                <div class="max-w-3xl">
                    <h1 class="text-5xl md:text-7xl font-black text-[#232f3e] mb-8 tracking-tighter">Documentation</h1>
                    <p class="text-xl text-[#545b64] mb-12 font-medium leading-relaxed">
                        Everything you need to build, scale, and manage your infrastructure on Serwin Systems. From
                        quickstart guides to deep-dive API references.
                    </p>
                    <div class="relative max-w-xl">
                        <div class="absolute inset-y-0 left-0 pl-4 flex items-center pointer-events-none">
                            <svg class="h-5 w-5 text-[#879196]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                                    d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                            </svg>
                        </div>
                        <input v-model="searchQuery" type="text" placeholder="Search documentation..."
                            class="w-full pl-12 pr-4 py-4 bg-white border-2 border-[#232f3e] focus:outline-none focus:border-[#ff9900] text-lg font-bold placeholder-[#879196] transition-all" />
                        <button v-if="searchQuery" @click="searchQuery = ''"
                            class="absolute inset-y-0 right-0 pr-4 flex items-center text-[#879196] hover:text-[#232f3e]">
                            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                                    d="M6 18L18 6M6 6l12 12" />
                            </svg>
                        </button>
                    </div>
                </div>
            </div>
        </section>

        <!-- Main Content Grid -->
        <main class="py-20 max-w-7xl mx-auto px-6">
            <div v-if="filteredDocs.length > 0" class="grid md:grid-cols-2 lg:grid-cols-3 gap-8">
                <div v-for="doc in filteredDocs" :key="doc.id" @click="navigateToDoc(doc)" :class="[
                    'border-2 p-10 transition-all flex flex-col items-start bg-white relative overflow-hidden',
                    doc.status === 'active' ? 'border-[#eaeded] hover:border-[#232f3e] cursor-pointer group' : 'border-[#eaeded]/50 opacity-60 cursor-not-allowed grayscale-[0.5]'
                ]">

                    <!-- Coming Soon Ribbon/Badge -->
                    <div v-if="doc.status === 'coming-soon'"
                        class="absolute top-4 right-4 bg-[#232f3e] text-white text-[10px] font-black px-2 py-1 uppercase tracking-tighter">
                        Coming Soon
                    </div>

                    <div class="text-[10px] font-black text-[#ff9900] uppercase tracking-widest mb-6">{{ doc.tag }}
                    </div>
                    <h3 class="text-2xl font-black text-[#232f3e] mb-4">{{ doc.title }}</h3>
                    <p class="text-[#545b64] mb-10 leading-relaxed font-medium">{{ doc.description }}</p>

                    <div v-if="doc.status === 'active'"
                        class="mt-auto text-[#0073bb] font-bold uppercase tracking-widest text-sm group-hover:text-[#ff9900] transition-colors">
                        {{ doc.linkText }}
                    </div>
                    <div v-else class="mt-auto text-[#879196] font-bold uppercase tracking-widest text-sm">
                        Locked
                    </div>
                </div>
            </div>
            <div v-else class="py-20 text-center">
                <div class="text-4xl font-black text-[#232f3e] mb-4">No results found</div>
                <p class="text-[#545b64] font-medium">Try searching for something else or browse our categories.</p>
            </div>
        </main>

        <!-- Footer -->
        <footer class="bg-[#fafafa] py-12 border-t border-[#eaeded]">
            <div class="max-w-7xl mx-auto px-6 text-center">
                <p class="text-[#879196] font-bold text-xs uppercase tracking-widest">© 2026 Serwin Systems Inc.
                    Documentation Portal.</p>
            </div>
        </footer>
    </div>
</template>

<style scoped>
@import url('https://fonts.googleapis.com/css2?family=Urbanist:wght@400;500;600;700;800;900&display=swap');

.font-urbanist {
    font-family: 'Urbanist', sans-serif;
}

.font-black {
    font-weight: 900;
}
</style>
