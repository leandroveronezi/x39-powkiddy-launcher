/* sdljoy: lista os joysticks que o SDL 1.2 (fork do X39) enxerga e imprime os eventos de cada um por N segundos.
 * Serve para descobrir qual índice do SDL é qual dispositivo (console, controle USB) antes de mapear jogadores.
 * Uso (na jaula, com os nós de entrada): sdljoy [segundos] */
#include <SDL.h>
#include <stdio.h>
#include <stdlib.h>

int main(int argc, char **argv)
{
   int secs = argc > 1 ? atoi(argv[1]) : 30;
   if (SDL_Init(SDL_INIT_JOYSTICK) < 0) { fprintf(stderr, "SDL_Init: %s\n", SDL_GetError()); return 1; }
   int n = SDL_NumJoysticks();
   printf("SDL_NumJoysticks = %d\n", n);
   for (int i = 0; i < n; i++)
   {
      SDL_Joystick *j = SDL_JoystickOpen(i);
      printf("  [%d] \"%s\"  eixos=%d botoes=%d hats=%d%s\n", i, SDL_JoystickName(i),
             j ? SDL_JoystickNumAxes(j) : -1, j ? SDL_JoystickNumButtons(j) : -1, j ? SDL_JoystickNumHats(j) : -1,
             j ? "" : "  (NÃO ABRIU)");
   }
   fflush(stdout);
   /* leitura direta do estado (não depende do subsistema de eventos de vídeo) */
   SDL_Joystick *js[16] = {0};
   int ax[16][8], bt[16][32];
   for (int i = 0; i < n && i < 16; i++)
   {
      js[i] = SDL_JoystickOpen(i);
      for (int a = 0; a < 8; a++) ax[i][a] = 0;
      for (int b = 0; b < 32; b++) bt[i][b] = 0;
   }
   Uint32 end = SDL_GetTicks() + secs * 1000;
   while (SDL_GetTicks() < end)
   {
      SDL_JoystickUpdate();
      for (int i = 0; i < n && i < 16; i++)
      {
         if (!js[i]) continue;
         int na = SDL_JoystickNumAxes(js[i]), nb = SDL_JoystickNumButtons(js[i]);
         for (int a = 0; a < na && a < 8; a++)
         {
            int v = SDL_JoystickGetAxis(js[i], a);
            if (abs(v - ax[i][a]) > 8000) { printf("%6u ms  joystick %d  EIXO %d = %d\n", SDL_GetTicks(), i, a, v); fflush(stdout); }
            ax[i][a] = v;
         }
         for (int b = 0; b < nb && b < 32; b++)
         {
            int v = SDL_JoystickGetButton(js[i], b);
            if (v != bt[i][b]) { printf("%6u ms  joystick %d  BOTAO %d  %s\n", SDL_GetTicks(), i, b, v ? "desce" : "sobe"); fflush(stdout); }
            bt[i][b] = v;
         }
      }
      SDL_Delay(5);
   }
   SDL_Quit();
   return 0;
}
