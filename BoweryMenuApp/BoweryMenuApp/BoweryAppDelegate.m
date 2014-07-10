//
//  BoweryAppDelegate.m
//  BoweryMenuApp
//
//  Created by Steve Kaliski on 7/10/14.
//  Copyright (c) 2014 Bowery. All rights reserved.
//

#import "BoweryAppDelegate.h"
#import <WebKit/WebKit.h>

@implementation BoweryAppDelegate

- (void)applicationDidFinishLaunching:(NSNotification *)aNotification
{
	NSURLRequest *request = [NSURLRequest requestWithURL:[NSURL URLWithString:@"http://localhost:32055/"]];
	[self.webView.mainFrame loadRequest:request];
}

@end
