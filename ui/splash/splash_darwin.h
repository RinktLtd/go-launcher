#ifndef SPLASH_DARWIN_H
#define SPLASH_DARWIN_H

void SplashSetIcon(const char *data, int length);
void SplashSetTitle(const char *title);
void SplashSetColor(float r, float g, float b);
void SplashShow(const char *status);
void SplashUpdate(double percent, const char *status);
void SplashHide(void);
void SplashError(const char *message);

#endif
