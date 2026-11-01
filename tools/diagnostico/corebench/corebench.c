/* corebench: mini-frontend libretro para calibrar um core SEM o RetroArch.
 * Carrega o core, roda N quadros e mede:
 *   - fps e taxa de áudio que o core declara (retro_get_system_av_info);
 *   - quantas amostras de áudio o core entrega por quadro (esperado: taxa/fps);
 *   - a velocidade do core sem freio (quadros por segundo) e o custo por quadro.
 * Se "amostras por quadro" bate com taxa/fps, o core mantém o tempo certo e basta o áudio sincronizar.
 * Não abre vídeo, som nem botões: só CPU. Uso: corebench <core.so> <rom|-> [quadros]  ("-" = só identifica o core) */
#define _GNU_SOURCE
#include <dlfcn.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include "libretro.h"

static unsigned long video_calls, audio_frames, audio_batches, dupe_calls;
static unsigned fmt = RETRO_PIXEL_FORMAT_0RGB1555;

static bool env_cb(unsigned cmd, void *data)
{
   switch (cmd)
   {
      case RETRO_ENVIRONMENT_SET_PIXEL_FORMAT: fmt = *(unsigned*)data; return true;
      case RETRO_ENVIRONMENT_GET_CAN_DUPE: *(bool*)data = true; return true;
      case RETRO_ENVIRONMENT_GET_SYSTEM_DIRECTORY: /* X39_SYSDIR: BIOS e dados do core (ex.: prboom.wad) */
      {
         const char *d = getenv("X39_SYSDIR");
         *(const char**)data = d ? d : "/tmp";
         return true;
      }
      case RETRO_ENVIRONMENT_GET_SAVE_DIRECTORY: *(const char**)data = "/tmp"; return true;
      case RETRO_ENVIRONMENT_GET_LOG_INTERFACE: return false;
      case RETRO_ENVIRONMENT_GET_VARIABLE_UPDATE: *(bool*)data = false; return true;
      case RETRO_ENVIRONMENT_SET_VARIABLES: return true;
      default: return false;
   }
}
static void video_cb(const void *d, unsigned w, unsigned h, size_t p) { video_calls++; if (!d) dupe_calls++; (void)w; (void)h; (void)p; }
static void audio_cb(int16_t l, int16_t r) { audio_frames++; audio_batches++; (void)l; (void)r; }
static size_t audio_batch_cb(const int16_t *d, size_t n) { audio_frames += n; audio_batches++; (void)d; return n; }
static void poll_cb(void) {}
static int16_t input_cb(unsigned p, unsigned d, unsigned i, unsigned id) { (void)p; (void)d; (void)i; (void)id; return 0; }

static double now(void) { struct timespec t; clock_gettime(CLOCK_MONOTONIC, &t); return t.tv_sec + t.tv_nsec / 1e9; }

int main(int argc, char **argv)
{
   if (argc < 3) { fprintf(stderr, "uso: %s <core.so> <rom> [quadros]\n", argv[0]); return 2; }
   int frames = argc > 3 ? atoi(argv[3]) : 600;
   void *h = dlopen(argv[1], RTLD_NOW);
   if (!h) { fprintf(stderr, "dlopen: %s\n", dlerror()); return 1; }
#define SYM(T, n) T n = (T)dlsym(h, #n); if (!n) { fprintf(stderr, "falta %s\n", #n); return 1; }
   typedef void (*f_env)(retro_environment_t);
   typedef void (*f_vid)(retro_video_refresh_t);
   typedef void (*f_aud)(retro_audio_sample_t);
   typedef void (*f_bat)(retro_audio_sample_batch_t);
   typedef void (*f_pol)(retro_input_poll_t);
   typedef void (*f_inp)(retro_input_state_t);
   typedef void (*f_void)(void);
   typedef void (*f_si)(struct retro_system_info*);
   typedef void (*f_av)(struct retro_system_av_info*);
   typedef bool (*f_load)(const struct retro_game_info*);
   SYM(f_env, retro_set_environment)
   SYM(f_vid, retro_set_video_refresh)
   SYM(f_aud, retro_set_audio_sample)
   SYM(f_bat, retro_set_audio_sample_batch)
   SYM(f_pol, retro_set_input_poll)
   SYM(f_inp, retro_set_input_state)
   SYM(f_void, retro_init)
   SYM(f_si, retro_get_system_info)
   SYM(f_av, retro_get_system_av_info)
   SYM(f_load, retro_load_game)
   SYM(f_void, retro_run)

   struct retro_system_info si; memset(&si, 0, sizeof si);
   retro_get_system_info(&si); /* a API permite chamar antes de retro_init (cores com BIOS abortam no init) */
   if (argc > 2 && strcmp(argv[2], "-") == 0)
   {
      printf("core: %s %s | extensões: %s | need_fullpath=%d block_extract=%d\n", si.library_name, si.library_version,
             si.valid_extensions ? si.valid_extensions : "(nenhuma)", si.need_fullpath, si.block_extract);
      return 0;
   }
   retro_set_environment(env_cb);
   retro_init();
   retro_set_video_refresh(video_cb); retro_set_audio_sample(audio_cb);
   retro_set_audio_sample_batch(audio_batch_cb); retro_set_input_poll(poll_cb); retro_set_input_state(input_cb);

   struct retro_game_info gi; memset(&gi, 0, sizeof gi);
   gi.path = argv[2];
   void *buf = NULL;
   if (!si.need_fullpath)
   {
      FILE *f = fopen(argv[2], "rb"); if (!f) { perror("rom"); return 1; }
      fseek(f, 0, SEEK_END); long n = ftell(f); fseek(f, 0, SEEK_SET);
      buf = malloc(n); if (fread(buf, 1, n, f) != (size_t)n) return 1; fclose(f);
      gi.data = buf; gi.size = n;
   }
   if (!retro_load_game(&gi)) { fprintf(stderr, "retro_load_game falhou\n"); return 1; }
   struct retro_system_av_info av; memset(&av, 0, sizeof av); retro_get_system_av_info(&av);

   for (int i = 0; i < 120; i++) retro_run(); /* aquecimento: sai do boot do jogo */
   video_calls = audio_frames = audio_batches = dupe_calls = 0;
   double t0 = now();
   for (int i = 0; i < frames; i++) retro_run();
   double dt = now() - t0;

   double expected = av.timing.sample_rate / av.timing.fps;
   double per = (double)audio_frames / frames;
   printf("core: %s %s\n", si.library_name, si.library_version);
   printf("declarado: %.3f quadros/s, áudio %.2f Hz, %ux%u\n", av.timing.fps, av.timing.sample_rate,
          av.geometry.base_width, av.geometry.base_height);
   printf("por quadro: %.2f amostras de áudio (esperado taxa/fps = %.2f) em %.2f chamadas de áudio; %.2f chamadas de vídeo (%lu repetidas)\n",
          per, expected, (double)audio_batches / frames, (double)video_calls / frames, dupe_calls);
   printf("coerência do tempo = %.4f (1.0000 = o core mantém taxa/fps)\n", per / expected);
   printf("sem freio: %.1f quadros/s (%.2f ms por quadro); com o tempo declarado, usaria %.0f%% de um núcleo\n",
          frames / dt, 1000.0 * dt / frames, 100.0 * (dt / frames) * av.timing.fps);
   return 0;
}
