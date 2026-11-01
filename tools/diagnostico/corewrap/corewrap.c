/* corewrap: core libretro "embrulho" para medir, DENTRO do RetroArch, quantas vezes por segundo o core
 * é de fato executado (retro_run), quantos quadros de vídeo ele entrega e quantas amostras de áudio.
 * Carrega o core real indicado em $X39_REALCORE e repassa tudo; só conta e escreve em stderr, 1 linha/s.
 * Uso (na jaula): -env X39_REALCORE=/app/cores/picodrive_libretro.so  e carregar este arquivo como core. */
#define _GNU_SOURCE
#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include "libretro.h"

static void *real;
static retro_video_refresh_t v_cb;
static retro_audio_sample_t a_cb;
static retro_audio_sample_batch_t ab_cb;
static retro_input_state_t in_cb;
static int in_prev[4][16];
static unsigned long runs, vids, aframes, total_runs;
static double t_last;

static double now(void) { struct timespec t; clock_gettime(CLOCK_MONOTONIC, &t); return t.tv_sec + t.tv_nsec / 1e9; }
static void *sym(const char *n)
{
   if (!real)
   {
      const char *p = getenv("X39_REALCORE");
      real = p ? dlopen(p, RTLD_NOW | RTLD_LOCAL) : NULL;
      if (!real) { fprintf(stderr, "[wrap] não abriu o core real: %s\n", dlerror()); exit(1); }
   }
   void *s = dlsym(real, n);
   if (!s) { fprintf(stderr, "[wrap] falta %s\n", n); exit(1); }
   return s;
}

/* entrada: registra cada botão do RetroPad que o RetroArch entrega ao core, por porta (0 = jogador 1, 1 = jogador 2) */
static void in_log(unsigned port, unsigned id, int p)
{
   static const char *nome[] = {"B","Y","SELECT","START","CIMA","BAIXO","ESQUERDA","DIREITA","A","X","L","R","L2","R2","L3","R3"};
   if (p != in_prev[port][id])
   {
      in_prev[port][id] = p;
      fprintf(stderr, "[wrap] ENTRADA porta %u (jogador %u): %s %s\n", port, port + 1, nome[id], p ? "desce" : "sobe");
      fflush(stderr);
   }
}

static unsigned long in_calls;
static int16_t in_wrap(unsigned port, unsigned dev, unsigned idx, unsigned id)
{
   int16_t v = in_cb ? in_cb(port, dev, idx, id) : 0;
   in_calls++;
   if (dev == RETRO_DEVICE_JOYPAD && port < 4)
   {
      if (id < 16) in_log(port, id, v != 0);
      else if (id == RETRO_DEVICE_ID_JOYPAD_MASK) /* o core lê todos os botões de uma vez */
         for (unsigned b = 0; b < 16; b++) in_log(port, b, (v >> b) & 1);
   }
   if (v != 0 && !(dev == RETRO_DEVICE_JOYPAD && port < 4 && (id < 16 || id == RETRO_DEVICE_ID_JOYPAD_MASK)))
   {
      static unsigned n;
      if (n++ < 40) { fprintf(stderr, "[wrap] ENTRADA-OUTRA porta=%u dispositivo=%u indice=%u id=%u valor=%d\n", port, dev, idx, id, v); fflush(stderr); }
   }
   return v;
}

static void v_wrap(const void *d, unsigned w, unsigned h, size_t p) { vids++; if (v_cb) v_cb(d, w, h, p); }
static void a_wrap(int16_t l, int16_t r) { aframes++; if (a_cb) a_cb(l, r); }
static size_t ab_wrap(const int16_t *d, size_t n) { aframes += n; return ab_cb ? ab_cb(d, n) : n; }

#define FWD0(ret, name) ret name(void) { return ((ret (*)(void))sym(#name))(); }
void retro_set_environment(retro_environment_t cb) { ((void (*)(retro_environment_t))sym("retro_set_environment"))(cb); }
void retro_set_video_refresh(retro_video_refresh_t cb) { v_cb = cb; ((void (*)(retro_video_refresh_t))sym("retro_set_video_refresh"))(v_wrap); }
void retro_set_audio_sample(retro_audio_sample_t cb) { a_cb = cb; ((void (*)(retro_audio_sample_t))sym("retro_set_audio_sample"))(a_wrap); }
void retro_set_audio_sample_batch(retro_audio_sample_batch_t cb) { ab_cb = cb; ((void (*)(retro_audio_sample_batch_t))sym("retro_set_audio_sample_batch"))(ab_wrap); }
void retro_set_input_poll(retro_input_poll_t cb) { ((void (*)(retro_input_poll_t))sym("retro_set_input_poll"))(cb); }
void retro_set_input_state(retro_input_state_t cb) { in_cb = cb; ((void (*)(retro_input_state_t))sym("retro_set_input_state"))(in_wrap); }
FWD0(void, retro_init)
FWD0(void, retro_deinit)
FWD0(unsigned, retro_api_version)
void retro_get_system_info(struct retro_system_info *i) { ((void (*)(struct retro_system_info*))sym("retro_get_system_info"))(i); }
void retro_get_system_av_info(struct retro_system_av_info *i)
{
   ((void (*)(struct retro_system_av_info*))sym("retro_get_system_av_info"))(i);
   fprintf(stderr, "[wrap] declarado: %.3f quadros/s, áudio %.2f Hz\n", i->timing.fps, i->timing.sample_rate);
}
void retro_set_controller_port_device(unsigned p, unsigned d) { ((void (*)(unsigned, unsigned))sym("retro_set_controller_port_device"))(p, d); }
FWD0(void, retro_reset)
size_t retro_serialize_size(void) { return ((size_t (*)(void))sym("retro_serialize_size"))(); }
bool retro_serialize(void *d, size_t n) { return ((bool (*)(void*, size_t))sym("retro_serialize"))(d, n); }
bool retro_unserialize(const void *d, size_t n) { return ((bool (*)(const void*, size_t))sym("retro_unserialize"))(d, n); }
FWD0(void, retro_cheat_reset)
void retro_cheat_set(unsigned i, bool e, const char *c) { ((void (*)(unsigned, bool, const char*))sym("retro_cheat_set"))(i, e, c); }
bool retro_load_game(const struct retro_game_info *g) { return ((bool (*)(const struct retro_game_info*))sym("retro_load_game"))(g); }
bool retro_load_game_special(unsigned t, const struct retro_game_info *g, size_t n) { return ((bool (*)(unsigned, const struct retro_game_info*, size_t))sym("retro_load_game_special"))(t, g, n); }
FWD0(void, retro_unload_game)
FWD0(unsigned, retro_get_region)
void *retro_get_memory_data(unsigned id) { return ((void *(*)(unsigned))sym("retro_get_memory_data"))(id); }
size_t retro_get_memory_size(unsigned id) { return ((size_t (*)(unsigned))sym("retro_get_memory_size"))(id); }

void retro_run(void)
{
   static void (*run)(void);
   if (!run) { run = (void (*)(void))sym("retro_run"); t_last = now(); }
   run();
   runs++; total_runs++;
   double t = now();
   if (t - t_last >= 1.0)
   {
      fprintf(stderr, "[wrap] t=%.1fs: retro_run=%lu/s  vídeo=%lu/s  áudio=%lu amostras/s (%.1f por run)  total=%lu  consultas-de-entrada=%lu\n",
              t - t_last, runs, vids, aframes, runs ? (double)aframes / runs : 0.0, total_runs, in_calls);
      in_calls = 0;
      fflush(stderr);
      runs = vids = aframes = 0; t_last = t;
   }
}
